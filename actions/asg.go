package actions

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cenkalti/backoff"

	"github.com/nest-egg/ami-replacer/log"
)

func (r *Replacement) replaceUnusedInstance(clst *cluster) (*ec2.StopInstancesOutput, error) {

	instances := clst.unusedInstances
	asgname := clst.asg.name
	num := clst.asg.size

	log.Info.Printf("stop instance %v", instances)

	params := &ec2.StopInstancesInput{
		DryRun:      aws.Bool(dryrun),
		InstanceIds: aws.StringSlice(instances),
	}
	result, err := r.asg.Ec2Api.StopInstances(params)
	if err != nil {
		return nil, err
	}

	describe := func() error {
		params := &ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(instances),
		}
		resp, err := r.asg.Ec2Api.DescribeInstances(params)
		if err != nil {
			return err
		}
		for idx, res := range resp.Reservations {
			log.Debug.Println("Reservation Id: ", *res.ReservationId, " Num Instances: ", len(res.Instances))
			for _, inst := range resp.Reservations[idx].Instances {
				code := inst.State.Code
				log.Info.Printf("current status code...: %d", *code)
				//0 (pending), 16 (running), 32 (shut-ting-down), 48 (terminated), 64 (stopping), and 80 (stopped).
				if *code != int64(48) && *code != int64(80) {
					return fmt.Errorf("There are still running instances: %v", *inst.InstanceId)
				}
			}
		}
		log.Info.Println("Successfully terminated all unused instance.")
		return nil
	}

	b := newExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 10)
	if err := backoff.Retry(describe, bf); err != nil {
		return nil, err
	}

	counter := func() error {
		asginfo, err := r.asgInfo(asgname)
		if err != nil {
			return err
		}
		size := len(asginfo.Instances)
		if size != num {
			return err
		}
		return nil
	}
	if err := backoff.Retry(counter, bf); err != nil {
		return nil, err
	}

	return result, err
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

func (r *Replacement) swapInstance(clst *cluster) (out *ec2.StopInstancesOutput, err error) {

	instances := clst.ecsInstance
	var targetAZ []string
	var wg sync.WaitGroup
	var emptyInstanceCount int
	asgname := clst.asg.name

	log.Info.Printf("replace ECS cluster instances with newest AMI: %s", clst.asg.newesetami)
	if err := r.deploy.FSM.Event("start"); err != nil {
		return nil, err
	}
	for _, inst := range instances {
		_, err := r.clearScaleinProtection(inst, asgname)
		if err != nil {
			return nil, err
		}
		if inst.RunningTasks == 0 && inst.PendingTasks == 0 {
			log.Info.Printf("empty ECS instances with newest AMI is detected: %s", inst.InstanceID)
			targetAZ = append(targetAZ, inst.AvailabilityZone)
			emptyInstanceCount++
		}
	}
	if emptyInstanceCount == 0 {
		return nil, fmt.Errorf("no empty isntances")
	}

	for _, az := range targetAZ {
		log.Info.Printf("replace ECS instances running in az: %s", az)
		for _, inst := range instances {
			wg.Add(1)
			_, errc := r.swap(inst, &wg, az, clst.asg.newesetami, clst.asg.name)
			err := <-errc
			if err != nil {
				return nil, err
			}
			log.Info.Println("successfully replaced ECS instances")
		}
		wg.Wait()
	}
	if err := r.deploy.FSM.Event("finish"); err != nil {
		return nil, err
	}
	return nil, nil
}

func (r *Replacement) swap(inst AsgInstance, wg *sync.WaitGroup, az string, imageid string, asgname string) (<-chan string, <-chan error) {
	out := make(chan string, 1)
	errc := make(chan error, 1)
	var stoptarget []string
	go func() {
		{
			log.Info.Printf("start replacing instances: %v", inst)
			if inst.RunningTasks != 0 && inst.ImageID != imageid {
				if inst.AvailabilityZone == az {
					log.Info.Printf("ECS instances %s is running obsolete AMI in target az: %v", inst.InstanceID, az)
					_, err := r.drainInstance(inst)
					if err != nil {
						errc <- fmt.Errorf("cannnot drain instance: %v", err)
					}
					stoptarget = append(stoptarget, inst.InstanceID)
					c := &cluster{
						unusedInstances: stoptarget,
						asg: asg{
							name: asgname,
						},
					}
					clustername := inst.Cluster
					output, err := r.replaceUnusedInstance(c)
					_ = output
					if err != nil {
						errc <- fmt.Errorf("cannnot stop instance: %v", err)
					}
					if err := r.waitTasksRunning(clustername); err != nil {
						errc <- err
					}
					log.Info.Printf("target ECS instances successfully stopped")
				}
			} else if inst.RunningTasks != 0 && inst.ImageID == imageid && inst.AvailabilityZone == az {
				log.Info.Printf("target ECS instances %s already runs newest AMI", inst.InstanceID)
			} else if inst.RunningTasks == 0 && inst.AvailabilityZone == az {
				log.Info.Printf("nothing to do. empty instance with new imageid: %v", inst.InstanceID)
			}
			out <- "done!"
			close(errc)
			close(out)
			defer wg.Done()
		}
	}()

	return out, errc
}

func (r *Replacement) waitTasksRunning(clustername string) error {

	var taskscount int64
	b := newExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 10)

	counter := func() error {
		status, err := r.clusterStatus(clustername)
		if err != nil {
			return err
		}
		for _, st := range status.ContainerInstances {
			if *st.Status == "ACTIVE" {
				taskscount += *st.RunningTasksCount
			}
		}
		if taskscount == 0 {
			return fmt.Errorf("waiting for RunningTasksCount >=1")
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
	log.Info.Printf("update asg %s size to %d", asgname, num)
	params := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName:             aws.String(asgname),
		DesiredCapacity:                  aws.Int64(desired),
		MaxSize:                          aws.Int64(desired),
		NewInstancesProtectedFromScaleIn: aws.Bool(true),
	}
	result, err := r.asg.AsgAPI.UpdateAutoScalingGroup(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Replacement) clearScaleinProtection(instance AsgInstance, asgname string) (*autoscaling.SetInstanceProtectionOutput, error) {

	params := &autoscaling.SetInstanceProtectionInput{
		AutoScalingGroupName: aws.String(asgname),
		InstanceIds: []*string{
			aws.String(instance.InstanceID),
		},
		ProtectedFromScaleIn: aws.Bool(false),
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
		asginfo, err := r.asgInfo(asgname)
		if err != nil {
			return fmt.Errorf("cannnot get Asg Info: %v", err)
		}
		size := len(asginfo.Instances)
		if size != num {
			return fmt.Errorf("there are still pending instances")
		}
		status, err := r.clusterStatus(clst.name)
		if err != nil {
			return err
		}
		for _, st := range status.ContainerInstances {
			log.Debug.Printf("current cluster size: %d", clst.size)
			log.Debug.Printf("dst size: %d", num)
			if *st.Status == "DRAINING" && clst.size > num {
				offset++
			}
		}
		if len(status.ContainerInstances)-offset != num {
			log.Info.Printf("ECS Cluster is still in pending status")
			log.Debug.Printf("current ecs cluster size: %d", clst.size)
			log.Debug.Printf("current offset: %d", offset)
			log.Debug.Printf("num of container instances: %d", len(status.ContainerInstances))
			return fmt.Errorf("Scaling operation has timed out")
		}
		return nil
	}
	if err := backoff.Retry(counter, bf); err != nil {
		return err
	}
	return nil
}

func (r *Replacement) ecsInstance(clustername string, asgname string, newestimage string, clustersize int) ([]AsgInstance, error) {

	var ecsInstance []AsgInstance
	status, err := r.clusterStatus(clustername)
	if err != nil {
		return nil, err
	}
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) && *st.Status == "ACTIVE" {
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, err
			}
			if imageid == newestimage {
				region, err := r.region(*st.Ec2InstanceId)
				if err != nil {
					return nil, fmt.Errorf("cannnot get region: %v", err)
				}
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          newestimage,
					Cluster:          clustername,
					ClusterSize:      clustersize,
					RunningTasks:     0,
					PendingTasks:     0,
					AvailabilityZone: region,
				}
				ecsInstance = append(ecsInstance, *instance)
			}
		} else if *st.RunningTasksCount == int64(1) && *st.Status == "ACTIVE" {
			region, err := r.region(*st.Ec2InstanceId)
			if err != nil {
				return nil, fmt.Errorf("cannnot get region: %v", err)
			}
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, fmt.Errorf("cannnot get ami id: %v", err)
			}
			if imageid != newestimage {
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          imageid,
					RunningTasks:     1,
					PendingTasks:     0,
					Cluster:          clustername,
					ClusterSize:      clustersize,
					AvailabilityZone: region,
				}
				ecsInstance = append(ecsInstance, *instance)
			} else if imageid == newestimage {
				return nil, fmt.Errorf("instance has been already running with newest images")
			}
		}
	}
	return ecsInstance, nil
}

func (r *Replacement) unusedInstance(clustername string, newestimage string) ([]string, error) {

	var unusedInstances []string
	status, err := r.clusterStatus(clustername)
	if err != nil {
		return nil, err
	}
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) && *st.Status == "ACTIVE" {
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, err
			}
			if imageid != newestimage {
				unusedInstances = append(unusedInstances, *st.Ec2InstanceId)
			}
		}
	}
	return unusedInstances, nil
}

func (r *Replacement) freeInstance(clustername string, asgname string, newestimage string, clustersize int) ([]AsgInstance, error) {

	var freeInstance []AsgInstance
	status, err := r.clusterStatus(clustername)
	if err != nil {
		return nil, err
	}
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) && *st.Status == "ACTIVE" {
			imageid, err := r.Ami(*st.Ec2InstanceId)
			if err != nil {
				return nil, err
			}
			if imageid == newestimage {
				region, err := r.region(*st.Ec2InstanceId)
				if err != nil {
					return nil, fmt.Errorf("cannnot get region: %v", err)
				}
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          newestimage,
					Cluster:          clustername,
					ClusterSize:      clustersize,
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
