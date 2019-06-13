package actions

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/cenkalti/backoff"
	"github.com/nest-egg/ami-replacer/log"
)

//state of asg
var state string

//ReplaceInstance get status of asg instances
func (replacer *Replacement) ReplaceInstance(asgname string, cluster string, dryrun bool, image string, owner string) (grp *autoscaling.Group, err error) {
	asgGroup, err := replacer.InfoAsg(asgname)
	if err != nil {
		return nil, err
	}
	clustername := cluster
	defaultClusterSize := len(asgGroup.Instances)
	newestimage, err := replacer.GetNewestAMI(owner, image)
	if err != nil {
		return nil, err
	}

	ecsInstance, err := replacer.getECSInstance(clustername, asgname, newestimage, defaultClusterSize)
	if err != nil {
		return nil, err
	}
	unusedInstances, err := replacer.getUnusedInstance(clustername, asgname, newestimage, defaultClusterSize)
	if err != nil {
		return nil, err
	}

	freeInstances, err := replacer.getFreeInstance(clustername, asgname, newestimage, defaultClusterSize)
	if err != nil {
		return nil, err
	}

	if len(unusedInstances) != 0 {
		if err := replacer.deploy.FSM.Event("start"); err != nil {
			return nil, err
		}
		output, err := replacer.replaceUnusedInstance(asgname, unusedInstances, dryrun)
		if err != nil {
			return nil, fmt.Errorf("cannnot stop unused instance: %v", err)
		}
		if err := replacer.deploy.FSM.Event("finish"); err != nil {
			return nil, err
		}
		unusedInstances, err = replacer.getUnusedInstance(clustername, asgname, newestimage, defaultClusterSize)
		if err != nil {
			return nil, err
		}
		freeInstances, err = replacer.getFreeInstance(clustername, asgname, newestimage, defaultClusterSize)
		if err != nil {
			return nil, err
		}
		log.Debug.Println(output)
	}

	state = replacer.deploy.FSM.Current()

	if len(freeInstances) == 0 && state == "closed" {
		log.Info.Println("cluster has no empty ECS instances")
		log.Info.Printf("extend the size of the cluster.. current size: %d", defaultClusterSize)
		if err := replacer.optimizeClusterSize(clustername, asgname, defaultClusterSize+1); err != nil {
			return nil, err
		}
		ecsInstance, err = replacer.getECSInstance(clustername, asgname, newestimage, defaultClusterSize)
		if err != nil {
			return nil, err
		}
		log.Info.Println(ecsInstance)
	}

	if len(ecsInstance) != 0 && state == "closed" {
		_, err := replacer.swapInstance(ecsInstance, newestimage, dryrun)
		if err != nil {
			return nil, err
		}
	}

	state = replacer.deploy.FSM.Current()
	if state != "closed" {
		return nil, fmt.Errorf("cluster is not steady state")
	} else if state == "closed" {
		if err := replacer.optimizeClusterSize(clustername, asgname, defaultClusterSize); err != nil {
			return nil, err
		}
		log.Info.Println("successfully recovered the size of the cluster")

	}
	return nil, nil
}

func (replacer *Replacement) getEcsInstanceArn(clustername string) (out []string, err error) {
	var instanceArns []string
	params := &ecs.ListContainerInstancesInput{
		Cluster: aws.String(clustername),
	}
	output, err := replacer.asg.EcsAPI.ListContainerInstances(params)
	if err != nil {
		return nil, err
	}
	for _, instance := range output.ContainerInstanceArns {
		instanceArns = append(instanceArns, aws.StringValue(instance))
	}
	return instanceArns, err
}

//EcsInstanceStatus returns container instance status.
func (replacer *Replacement) EcsInstanceStatus(clustername string, instances []string) (out *ecs.DescribeContainerInstancesOutput, err error) {

	params := &ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(clustername),
		ContainerInstances: aws.StringSlice(instances),
	}
	log.Debug.Println(params)
	output, err := replacer.asg.EcsAPI.DescribeContainerInstances(params)
	if err != nil {
		return nil, err
	}
	return output, err
}

func (replacer *Replacement) replaceUnusedInstance(asgname string, instances []string, dryrun bool) (*ec2.StopInstancesOutput, error) {
	params := &ec2.StopInstancesInput{
		DryRun:      aws.Bool(dryrun),
		InstanceIds: aws.StringSlice(instances),
	}

	asginfo, err := replacer.InfoAsg(asgname)
	if err != nil {
		return nil, err
	}

	num := len(asginfo.Instances)
	log.Info.Printf("num of asg instances before replace: %v", num)
	log.Info.Printf("stop instance %v", instances)
	result, err := replacer.asg.Ec2Api.StopInstances(params)
	if err != nil {
		return nil, err
	}

	//describe instance status
	describe := func() error {
		params := &ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(instances),
		}
		resp, err := replacer.asg.Ec2Api.DescribeInstances(params)
		if err != nil {
			return err
		}
		for idx, res := range resp.Reservations {
			log.Println("  > Reservation Id", *res.ReservationId, " Num Instances: ", len(res.Instances))
			for _, inst := range resp.Reservations[idx].Instances {
				code := inst.State.Code
				log.Info.Printf("status code: %d", *code)
				//0 (pending), 16 (running), 32 (shut-ting-down), 48 (terminated), 64 (stopping), and 80 (stopped).
				if *code != int64(48) && *code != int64(80) {
					return fmt.Errorf("still running instance: %v", *inst.InstanceId)
				}
			}
		}
		log.Info.Println("successfully terminated all unused instance.")
		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Duration(10) * time.Second
	b.MaxInterval = time.Duration(30) * time.Second
	b.MaxElapsedTime = time.Duration(300) * time.Second
	bf := backoff.WithMaxRetries(b, 10)
	if err := backoff.Retry(describe, bf); err != nil {
		return nil, err
	}

	counter := func() error {
		asginfo, err := replacer.InfoAsg(asgname)
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

func (replacer *Replacement) getRegion(instancid string) (region string, err error) {
	params := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instancid),
		},
	}
	result, err := replacer.asg.Ec2Api.DescribeInstances(params)
	region = *result.Reservations[0].Instances[0].Placement.AvailabilityZone
	return region, nil
}

func (replacer *Replacement) swapInstance(instances []AsgInstance, imageid string, dryrun bool) (out *ec2.StopInstancesOutput, err error) {

	var targetAZ []string
	var wg sync.WaitGroup
	var emptyInstanceCount int
	asgname := instances[0].Asgname

	log.Info.Printf("replace ECS cluster instances with newest AMI: %s", imageid)
	if err := replacer.deploy.FSM.Event("start"); err != nil {
		return nil, err
	}
	for _, inst := range instances {
		_, err := replacer.clearScaleinProtection(inst)
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
			_, errc := replacer.swap(inst, &wg, az, imageid, asgname, dryrun)
			err := <-errc
			if err != nil {
				return nil, err
			}
			log.Info.Println("successfully replaced ECS instances")
		}
		wg.Wait()
	}
	if err := replacer.deploy.FSM.Event("finish"); err != nil {
		return nil, err
	}
	return nil, nil
}

func (replacer *Replacement) swap(inst AsgInstance, wg *sync.WaitGroup, az string, imageid string, asgname string, dryrun bool) (<-chan string, <-chan error) {
	out := make(chan string, 1)
	errc := make(chan error, 1)
	var stoptarget []string
	go func() {
		{
			log.Info.Printf("start replacing instances: %v", inst)
			if inst.RunningTasks != 0 && inst.ImageID != imageid {
				if inst.AvailabilityZone == az {
					log.Info.Printf("ECS instances %s is running obsolete AMI in target az: %v", inst.InstanceID, az)
					_, err := replacer.drainInstance(inst)
					if err != nil {
						errc <- fmt.Errorf("cannnot drain instance: %v", err)
					}
					stoptarget = append(stoptarget, inst.InstanceID)
					output, err := replacer.replaceUnusedInstance(asgname, stoptarget, dryrun)
					_ = output
					if err != nil {
						errc <- fmt.Errorf("cannnot stop instance: %v", err)
					}
					log.Info.Printf("target ECS instances successfully stopped")
				}
			} else if inst.RunningTasks != 0 && inst.ImageID == imageid && inst.AvailabilityZone == az {
				log.Info.Printf("target ECS instances %s already runs newest AMI", inst.InstanceID)
			} else if inst.RunningTasks == 0 && inst.AvailabilityZone == az {
				log.Info.Printf("empty instance with new imageid: %v", inst.InstanceID)
			}
			out <- "done!"
			close(errc)
			close(out)
			defer wg.Done()
		}
	}()

	return out, errc
}

func (replacer *Replacement) drainInstance(inst AsgInstance) (*ecs.UpdateContainerInstancesStateOutput, error) {

	params := &ecs.UpdateContainerInstancesStateInput{
		Cluster: aws.String(inst.Cluster),
		ContainerInstances: []*string{
			aws.String(inst.InstanceArn),
		},
		Status: aws.String("DRAINING"),
	}
	result, err := replacer.asg.EcsAPI.UpdateContainerInstancesState(params)
	if err != nil {
		return nil, err
	}
	log.Info.Printf("ECS instances %s has been successfully drained: %v", inst.InstanceID)
	return result, nil
}

func (replacer *Replacement) updateASG(asgname string, num int) (*autoscaling.UpdateAutoScalingGroupOutput, error) {

	desired := int64(num)
	log.Info.Printf("update asg %s size to %d", asgname, num)
	params := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName:             aws.String(asgname),
		DesiredCapacity:                  aws.Int64(desired),
		MaxSize:                          aws.Int64(desired),
		NewInstancesProtectedFromScaleIn: aws.Bool(true),
	}
	result, err := replacer.asg.AsgAPI.UpdateAutoScalingGroup(params)
	if err != nil {
		return nil, fmt.Errorf("failed to scale out asg: %v", err)
	}
	return result, nil
}

func (replacer *Replacement) clearScaleinProtection(instance AsgInstance) (*autoscaling.SetInstanceProtectionOutput, error) {

	params := &autoscaling.SetInstanceProtectionInput{
		AutoScalingGroupName: aws.String(instance.Asgname),
		InstanceIds: []*string{
			aws.String(instance.InstanceID),
		},
		ProtectedFromScaleIn: aws.Bool(false),
	}
	result, err := replacer.asg.AsgAPI.SetInstanceProtection(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (replacer *Replacement) optimizeClusterSize(clustername string, asgname string, num int) error {

	var offset int
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Duration(10) * time.Second
	b.MaxInterval = time.Duration(120) * time.Second
	b.MaxElapsedTime = time.Duration(600) * time.Second
	bf := backoff.WithMaxRetries(b, 10)

	_, err := replacer.updateASG(asgname, num)
	if err != nil {
		return fmt.Errorf("failed to update asg size: %v", err)
	}
	counter := func() error {
		asginfo, err := replacer.InfoAsg(asgname)
		if err != nil {
			return fmt.Errorf("cannnot get Asg Info: %v", err)
		}
		size := len(asginfo.Instances)
		if size != num {
			return fmt.Errorf("there are still pending instances")
		}
		status, err := replacer.getClusterStatus(clustername)
		if err != nil {
			return err
		}
		for _, st := range status.ContainerInstances {
			if *st.Status == "DRAINING" {
				offset++
			}
		}
		if len(status.ContainerInstances)-offset != num {
			log.Info.Printf("ECS Cluster is still in pending status")
			return fmt.Errorf("Scaling operation has timed out")
		}
		return nil
	}
	if err := backoff.Retry(counter, bf); err != nil {
		return err
	}
	return nil
}

func (replacer *Replacement) getClusterStatus(clustername string) (*ecs.DescribeContainerInstancesOutput, error) {
	arns, err := replacer.getEcsInstanceArn(clustername)
	if err != nil {
		return nil, fmt.Errorf("cannnot get instance arn: %v", err)
	}
	status, err := replacer.EcsInstanceStatus(clustername, arns)
	if err != nil {
		return nil, fmt.Errorf("cannnot get ecs status : %v", err)
	}
	return status, nil
}

func (replacer *Replacement) getECSInstance(clustername string, asgname string, newestimage string, defaultClusterSize int) ([]AsgInstance, error) {

	var ecsInstance []AsgInstance
	status, err := replacer.getClusterStatus(clustername)
	if err != nil {
		return nil, err
	}
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) {
			imageid, err := replacer.AmiAsg(*st.Ec2InstanceId)
			if err != nil {
				return nil, err
			}
			if imageid == newestimage {
				region, err := replacer.getRegion(*st.Ec2InstanceId)
				if err != nil {
					return nil, fmt.Errorf("cannnot get region: %v", err)
				}
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          newestimage,
					Cluster:          clustername,
					ClusterSize:      defaultClusterSize,
					RunningTasks:     0,
					PendingTasks:     0,
					AvailabilityZone: region,
					Asgname:          asgname,
				}
				ecsInstance = append(ecsInstance, *instance)
			}
		} else if *st.RunningTasksCount == int64(1) {
			region, err := replacer.getRegion(*st.Ec2InstanceId)
			if err != nil {
				return nil, fmt.Errorf("cannnot get region: %v", err)
			}
			imageid, err := replacer.AmiAsg(*st.Ec2InstanceId)
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
					ClusterSize:      defaultClusterSize,
					AvailabilityZone: region,
					Asgname:          asgname,
				}
				ecsInstance = append(ecsInstance, *instance)
			} else if imageid == newestimage {
				return nil, fmt.Errorf("instance has been already running with newest images")
			}
		}
	}
	return ecsInstance, nil
}

func (replacer *Replacement) getUnusedInstance(clustername string, asgname string, newestimage string, defaultClusterSize int) ([]string, error) {

	var unusedInstances []string
	status, err := replacer.getClusterStatus(clustername)
	if err != nil {
		return nil, err
	}
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) {
			imageid, err := replacer.AmiAsg(*st.Ec2InstanceId)
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

func (replacer *Replacement) getFreeInstance(clustername string, asgname string, newestimage string, defaultClusterSize int) ([]AsgInstance, error) {

	var freeInstance []AsgInstance
	status, err := replacer.getClusterStatus(clustername)
	if err != nil {
		return nil, err
	}
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) {
			imageid, err := replacer.AmiAsg(*st.Ec2InstanceId)
			if err != nil {
				return nil, err
			}
			if imageid == newestimage {
				region, err := replacer.getRegion(*st.Ec2InstanceId)
				if err != nil {
					return nil, fmt.Errorf("cannnot get region: %v", err)
				}
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          newestimage,
					Cluster:          clustername,
					ClusterSize:      defaultClusterSize,
					RunningTasks:     0,
					PendingTasks:     0,
					AvailabilityZone: region,
					Asgname:          asgname,
				}
				freeInstance = append(freeInstance, *instance)
			}
		}
	}
	return freeInstance, nil
}