package actions

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cenkalti/backoff"

	"github.com/nest-egg/ami-replacer/log"
	"golang.org/x/xerrors"
)

func (r *Replacement) replaceUnusedInstance(clst *cluster) (*ec2.StopInstancesOutput, error) {

	instances := clst.unusedInstances
	asgname := clst.asg.name
	num := clst.asg.size

	log.Logger.Infof("stop instance %v", instances)

	params := &ec2.StopInstancesInput{
		DryRun:      aws.Bool(dryrun),
		InstanceIds: aws.StringSlice(instances),
	}
	result, err := r.asg.Ec2Api.StopInstances(params)
	if err != nil {
		return nil, xerrors.Errorf("failed to stop instances: %w", err)
	}

	describe := func() error {
		params := &ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(instances),
		}
		resp, err := r.asg.Ec2Api.DescribeInstances(params)
		if err != nil {
			return xerrors.Errorf("failed to describer instances: %w", err)
		}
		for idx, res := range resp.Reservations {
			log.Logger.Debugf("Reservation Id: %s  Num Instances: %s", *res.ReservationId, len(res.Instances))
			for _, inst := range resp.Reservations[idx].Instances {
				code := inst.State.Code
				log.Logger.Infof("current status code...: %d", *code)
				//0 (pending), 16 (running), 32 (shut-ting-down), 48 (terminated), 64 (stopping), and 80 (stopped).
				if *code != int64(48) && *code != int64(80) {
					return xerrors.New("There are still running instances")
				}
			}
		}
		log.Logger.Info("Successfully terminated all unused instance.")
		return nil
	}

	b := newExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 10)
	if err := backoff.Retry(describe, bf); err != nil {
		return nil, xerrors.New("retry has timed out")
	}

	counter := func() error {
		asginfo, err := r.asgInfo(asgname)
		if err != nil {
			return xerrors.Errorf("failed to get asginfo: %w", err)
		}
		size := len(asginfo.Instances)
		log.Logger.Debugf("asg size= %d", size)
		log.Logger.Debugf("ecs cluster size= %d", num)
		if size != num {
			return xerrors.New("still waiting...")
		}
		return nil
	}
	if err := backoff.Retry(counter, bf); err != nil {
		return nil, xerrors.New("retry has timed out")
	}

	return result, nil
}

func (r *Replacement) region(instancid string) (region string, err error) {
	params := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instancid),
		},
	}
	result, err := r.asg.Ec2Api.DescribeInstances(params)
	region = *result.Reservations[0].Instances[0].Placement.AvailabilityZone
	return region, nil
}

func (r *Replacement) swapInstance(clst *cluster) error {

	instances := clst.ecsInstance
	var wg sync.WaitGroup
	var emptyInstanceCount int
	asgname := clst.asg.name

	log.Logger.Infof("replace ECS cluster instances with newest AMI: %s", clst.asg.newestami)
	if err := r.deploy.FSM.Event("start"); err != nil {
		return xerrors.Errorf("failed to enter state: %w", err)
	}
	for _, inst := range instances {
		_, err := r.clearScaleinProtection(inst.InstanceID, asgname)
		if err != nil {
			return xerrors.Errorf("failed to disable scale in protection: %w", err)
		}
		if inst.RunningTasks == 0 && inst.PendingTasks == 0 {
			log.Logger.Infof("empty ECS instances with newest AMI is detected: %s", inst.InstanceID)
			emptyInstanceCount++
		}
	}
	if emptyInstanceCount == 0 {
		return xerrors.New("no empty isntances")
	}

	for _, inst := range instances {
		wg.Add(1)
		_, errc := r.swap(inst, &wg, clst.asg)
		err := <-errc
		if err != nil {
			return xerrors.Errorf("failed to replace instances: %w", err)
		}
		log.Logger.Info("successfully replaced instances!")
	}
	wg.Wait()
	if err := r.deploy.FSM.Event("finish"); err != nil {
		return xerrors.Errorf("failed to enter state", err)
	}
	return nil
}

func (r *Replacement) swap(inst AsgInstance, wg *sync.WaitGroup, asg asg) (<-chan string, <-chan error) {
	out := make(chan string, 1)
	errc := make(chan error, 1)
	var stoptarget []string
	go func() {
		{
			log.Logger.Infof("start replacing instances: %v", inst)
			if inst.RunningTasks != 0 && inst.ImageID != asg.newestami {
				log.Logger.Infof("ECS instances %s is running obsolete AMI", inst.InstanceID)
				_, err := r.drainInstance(inst)
				if err != nil {
					errc <- xerrors.Errorf("cannnot drain instance: %w", err)
				}
				stoptarget = append(stoptarget, inst.InstanceID)
				c := &cluster{
					unusedInstances: stoptarget,
					asg:             asg,
				}
				clustername := inst.Cluster
				if err := r.waitTasksRunning(clustername, asg.name); err != nil {
					errc <- xerrors.Errorf("waiter has returned error: %w", err)
				}
				output, err := r.replaceUnusedInstance(c)
				_ = output
				if err != nil {
					errc <- xerrors.Errorf("failed to replace unused instance: %w", err)
				}
				if err := r.waitTasksRunning(clustername, asg.name); err != nil {
					errc <- xerrors.Errorf("waiter has returned error: %w", err)
				}
				log.Logger.Infof("target ECS instances successfully stopped")
			} else if inst.RunningTasks != 0 && inst.ImageID == asg.newestami {
				log.Logger.Infof("target ECS instances %s already runs newest AMI", inst.InstanceID)
			} else if inst.RunningTasks == 0 {
				log.Logger.Infof("nothing to do. empty instance with the newest ami: %v", inst.InstanceID)
			}
			out <- "done!"
			close(errc)
			close(out)
			defer wg.Done()
		}
	}()

	return out, errc
}

func (r *Replacement) waitTasksRunning(clustername string, asgname string) error {

	var taskscount int64
	b := newShortExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 100)

	counter := func() error {
		taskscount = 0
		status, err := r.clusterStatus(clustername)
		if err != nil {
			return err
		}
		for _, st := range status.ContainerInstances {
			if *st.Status == "ACTIVE" {
				taskscount += *st.RunningTasksCount
				if *st.RunningTasksCount != int64(0) {
					r.setScaleinProtection(*st.Ec2InstanceId, asgname)
				} else if *st.RunningTasksCount == int64(0) {
					r.clearScaleinProtection(*st.Ec2InstanceId, asgname)
				}
			} else if *st.Status != "ACTIVE" {
				r.clearScaleinProtection(*st.Ec2InstanceId, asgname)
			}
		}
		if taskscount == 0 {
			return xerrors.New("waiting for RunningTasksCount >=1")
		}

		return nil
	}

	if err := backoff.Retry(counter, bf); err != nil {
		return err
	}

	return nil
}

func (r *Replacement) updateASG(asgname string, num int) (*autoscaling.UpdateAutoScalingGroupOutput, error) {

	desired := int64(num)
	log.Logger.Infof("update asg %s size to %d", asgname, num)
	params := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName:             aws.String(asgname),
		DesiredCapacity:                  aws.Int64(desired),
		MinSize:                          aws.Int64(desired),
		NewInstancesProtectedFromScaleIn: aws.Bool(false),
	}
	result, err := r.asg.AsgAPI.UpdateAutoScalingGroup(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Replacement) clearScaleinProtection(instanceid string, asgname string) (*autoscaling.SetInstanceProtectionOutput, error) {

	params := &autoscaling.SetInstanceProtectionInput{
		AutoScalingGroupName: aws.String(asgname),
		InstanceIds: []*string{
			aws.String(instanceid),
		},
		ProtectedFromScaleIn: aws.Bool(false),
	}
	result, err := r.asg.AsgAPI.SetInstanceProtection(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Replacement) setScaleinProtection(instanceid string, asgname string) (*autoscaling.SetInstanceProtectionOutput, error) {

	params := &autoscaling.SetInstanceProtectionInput{
		AutoScalingGroupName: aws.String(asgname),
		InstanceIds: []*string{
			aws.String(instanceid),
		},
		ProtectedFromScaleIn: aws.Bool(true),
	}
	result, err := r.asg.AsgAPI.SetInstanceProtection(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Replacement) optimizeClusterSize(clst *cluster, num int) error {

	var offset int
	asgname := clst.asg.name
	b := newExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 10)

	status, err := r.clusterStatus(clst.name)
	if err != nil {
		return err
	}
	clst.size = len(status.ContainerInstances)
	for _, st := range status.ContainerInstances {
		if *st.Status == "DRAINING" {
			clst.size = clst.size - 1
		}
	}

	output, err := r.updateASG(asgname, num)
	_ = output
	if err != nil {
		return err
	}
	counter := func() error {
		offset = 0
		asginfo, err := r.asgInfo(asgname)
		if err != nil {
			return xerrors.Errorf("cannnot get Asg Info: %w", err)
		}
		size := len(asginfo.Instances)
		if size != num {
			return xerrors.New("there are still pending instances")
		}
		status, err := r.clusterStatus(clst.name)
		if err != nil {
			return xerrors.Errorf("failed to get cluster status: %w", err)
		}
		for _, st := range status.ContainerInstances {
			log.Logger.Debugf("current cluster size: %d", clst.size)
			log.Logger.Debugf("dst size: %d", num)
			if *st.Status == "DRAINING" && len(status.ContainerInstances) > num {
				offset++
			}
		}
		if len(status.ContainerInstances)-offset != num {
			log.Logger.Infof("ECS Cluster is still in pending status")
			log.Logger.Debugf("current ecs cluster size: %d", clst.size)
			log.Logger.Debugf("current offset: %d", offset)
			log.Logger.Debugf("num of container instances: %d", len(status.ContainerInstances))
			return xerrors.New("Scaling operation has timed out")
		}
		return nil
	}
	if err := backoff.Retry(counter, bf); err != nil {
		return xerrors.New("retry failure")
	}
	return nil
}

func (r *Replacement) ecsInstance(clst *cluster) ([]AsgInstance, error) {

	var ecsInstance []AsgInstance
	var count int
	var len int
	status, err := r.clusterStatus(clst.name)
	if err != nil {
		return nil, xerrors.Errorf("failed to get cluster status: %w", err)
	}

	for _, st := range status.ContainerInstances {
		if *st.Status == "ACTIVE" && *st.AgentConnected == true {
			len++
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, xerrors.Errorf("failed to get ami id: %w", err)
			}
			if imageid == clst.asg.newestami {
				count++
			}
		}
	}

	if count == len {
		return nil, fmt.Errorf("All instances have been already running with newest images")
	}

	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) && *st.Status == "ACTIVE" {
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, xerrors.Errorf("failed to get ami id: %w", err)
			}
			if imageid == clst.asg.newestami {
				region, err := r.region(*st.Ec2InstanceId)
				if err != nil {
					return nil, xerrors.Errorf("cannnot get region: %w", err)
				}
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          clst.asg.newestami,
					Cluster:          clst.name,
					RunningTasks:     0,
					PendingTasks:     0,
					AvailabilityZone: region,
				}
				ecsInstance = append(ecsInstance, *instance)
			}
		} else if *st.RunningTasksCount == int64(1) && *st.Status == "ACTIVE" {
			region, err := r.region(*st.Ec2InstanceId)
			if err != nil {
				return nil, xerrors.Errorf("cannnot get region: %w", err)
			}
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, xerrors.Errorf("cannnot get ami id: %w", err)
			}
			if imageid != clst.asg.newestami {
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          imageid,
					RunningTasks:     1,
					PendingTasks:     0,
					Cluster:          clst.name,
					AvailabilityZone: region,
				}
				ecsInstance = append(ecsInstance, *instance)
			} else if imageid == clst.asg.newestami {
				log.Logger.Infof("instance  %v has been already running with newest images", *st.Ec2InstanceId)
			}
		}
	}
	return ecsInstance, nil
}

func (r *Replacement) unusedInstance(clst *cluster) ([]string, error) {

	var unusedInstances []string
	status, err := r.clusterStatus(clst.name)
	if err != nil {
		return nil, err
	}
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) && *st.Status == "ACTIVE" {
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, err
			}
			if imageid != clst.asg.name {
				unusedInstances = append(unusedInstances, *st.Ec2InstanceId)
			}
		}
	}
	return unusedInstances, nil
}

func (r *Replacement) freeInstance(clst *cluster) ([]AsgInstance, error) {

	var freeInstance []AsgInstance
	status, err := r.clusterStatus(clst.name)
	if err != nil {
		return nil, xerrors.Errorf("failed to get cluster status: %w", err)
	}
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) && *st.Status == "ACTIVE" {
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, xerrors.Errorf("failed to get ami id: %w", err)
			}
			if imageid == clst.asg.newestami {
				region, err := r.region(*st.Ec2InstanceId)
				if err != nil {
					return nil, xerrors.Errorf("cannnot get region: %w", err)
				}
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          clst.asg.newestami,
					Cluster:          clst.name,
					RunningTasks:     0,
					PendingTasks:     0,
					AvailabilityZone: region,
				}
				freeInstance = append(freeInstance, *instance)
			}
		}
	}
	return freeInstance, nil
}
