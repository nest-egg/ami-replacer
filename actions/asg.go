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
		log.Fatalf("cannnot get Asg Info: %v", err)
	}
	var ecsInstance []AsgInstance

	var unusedInstances []string

	var freeInstances []string

	clustername := cluster
	ids := Ids(asgGroup)
	log.Debug.Println(ids)

	instancearns, err := replacer.getEcsInstanceArn(clustername)
	if err != nil {
		return nil, fmt.Errorf("cannnot get instance arn: %v", err)
	}
	status, err := replacer.EcsInstanceStatus(clustername, instancearns)
	if err != nil {
		return nil, fmt.Errorf("cannnot get ecs status : %v", err)
	}

	log.Debug.Println(status)

	newestimage, err := replacer.GetNewestAMI(owner, image)
	if err != nil {
		return nil, fmt.Errorf("cannnot get newestami id: %v", err)
	}
	log.Debug.Println(newestimage)
	//replace unused instance
	for _, st := range status.ContainerInstances {
		if *st.RunningTasksCount == int64(0) && *st.PendingTasksCount == int64(0) {
			imageid, err := replacer.AmiAsg(*st.Ec2InstanceId)
			if err != nil {
				return nil, fmt.Errorf("cannnot get ami id: %v", err)
			}
			if imageid != newestimage {
				unusedInstances = append(unusedInstances, *st.Ec2InstanceId)
			} else if imageid == newestimage {
				region, err := replacer.getRegion(*st.Ec2InstanceId)
				if err != nil {
					return nil, fmt.Errorf("cannnot get region: %v", err)
				}
				freeInstances = append(freeInstances, *st.Ec2InstanceId)
				instance := &AsgInstance{
					InstanceID:       *st.Ec2InstanceId,
					InstanceArn:      *st.ContainerInstanceArn,
					ImageID:          newestimage,
					Cluster:          clustername,
					RunningTasks:     0,
					PendingTasks:     0,
					AvailabilityZone: region,
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
			instance := &AsgInstance{
				InstanceID:       *st.Ec2InstanceId,
				InstanceArn:      *st.ContainerInstanceArn,
				ImageID:          imageid,
				RunningTasks:     1,
				PendingTasks:     0,
				Cluster:          clustername,
				AvailabilityZone: region,
			}
			ecsInstance = append(ecsInstance, *instance)
		}
	}
	log.Debug.Println(ecsInstance)
	log.Debug.Println(unusedInstances)
	if len(unusedInstances) != 0 {
		output, err := replacer.replaceUnusedInstance(asgname, unusedInstances, dryrun)
		if err != nil {
			return nil, fmt.Errorf("cannnot stop unused instance: %v", err)
		}
		log.Debug.Println(output)
	}

	state = replacer.deploy.FSM.Current()

	if len(ecsInstance) != 0 && state == "closed" {
		output, err := replacer.swapInstance(ecsInstance, newestimage, dryrun, asgname)
		if err != nil {
			return nil, fmt.Errorf("failed to replace instance: %v", err)
		}
		log.Debug.Println(output)
	}

	state = replacer.deploy.FSM.Current()
	if state != "closed" {
		return nil, fmt.Errorf("cluster is not steady state")
	}
	return nil, nil
}

//Ids return instance ids
func Ids(grp *autoscaling.Group) (ids []string) {
	instances := grp.Instances
	for _, entry := range instances {
		ids = append(ids, *entry.InstanceId)
	}
	return
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

	starterr := replacer.deploy.FSM.Event("start")
	if starterr != nil {
		return nil, fmt.Errorf("failed to set state %v", starterr)
	}
	log.Debug.Println(replacer.deploy.FSM.Current())

	asginfo, err := replacer.InfoAsg(asgname)
	if err != nil {
		return nil, fmt.Errorf("cannnot get Asg Info: %v", err)
	}
	num := len(asginfo.Instances)
	log.Debug.Printf("num of asg instances before replace: %v", num)
	result, err := replacer.asg.Ec2Api.StopInstances(params)
	if err != nil {
		return nil, err
	}

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
				//0 (pending), 16 (running), 32 (shut-ting-down), 48 (terminated), 64 (stopping), and 80 (stopped).
				if *code != int64(80) {
					return fmt.Errorf("still running instance: %v", inst.InstanceId)
				}
			}
		}
		log.Info.Println("successfully stopped instance.")
		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Duration(10) * time.Second
	b.MaxInterval = time.Duration(30) * time.Second
	b.MaxElapsedTime = time.Duration(120) * time.Second
	bf := backoff.WithMaxRetries(b, 4)
	retryerr := backoff.Retry(describe, bf)
	if retryerr != nil {
		return nil, retryerr
	}

	counter := func() error {
		asginfo, err := replacer.InfoAsg(asgname)
		if err != nil {
			return fmt.Errorf("cannnot get Asg Info: %v", err)
		}
		size := len(asginfo.Instances)
		if size != num {
			return fmt.Errorf("still pending instance")
		}
		return nil
	}
	retryerr2 := backoff.Retry(counter, bf)
	if retryerr2 != nil {
		return nil, retryerr2
	}

	finisherr := replacer.deploy.FSM.Event("finish")
	if finisherr != nil {
		return nil, fmt.Errorf("failed to set state %v", finisherr)
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

func (replacer *Replacement) swapInstance(instances []AsgInstance, imageid string, dryrun bool, asgname string) (out *ec2.StopInstancesOutput, err error) {

	var targetAZ []string
	var wg sync.WaitGroup
	var emptyInstanceCount int
	var stoptarget []string

	log.Info.Printf("replace ECS cluster instances with newest AMI: %s", imageid)
	for _, inst := range instances {
		if inst.RunningTasks == 0 && inst.PendingTasks == 0 {
			log.Info.Printf("empty ECS instances with newest AMI is detected: %s", inst.InstanceID)
			targetAZ = append(targetAZ, inst.AvailabilityZone)
			emptyInstanceCount++
		}
	}
	if emptyInstanceCount == 0 {
		log.Info.Println("cluster has no empty ECS instances.")
		return nil, nil
	}

	for _, az := range targetAZ {
		log.Info.Printf("replace ECS instances running in az: %s", az)
		for _, inst := range instances {
			wg.Add(1)
			go func(inst AsgInstance) {
				if inst.RunningTasks == 1 && inst.ImageID != imageid {
					if inst.AvailabilityZone == az {
						log.Info.Printf("ECS instances %s is running obsolete AMI in target az: %v", inst.InstanceID, az)
						params := &ecs.UpdateContainerInstancesStateInput{
							Cluster: aws.String(inst.Cluster),
							ContainerInstances: []*string{
								aws.String(inst.InstanceArn),
							},
							Status: aws.String("DRAINING"),
						}
						result, err := replacer.asg.EcsAPI.UpdateContainerInstancesState(params)
						if err != nil {
							log.Fatalf("failed to drain instance: %v", err)
						}
						log.Info.Printf("ECS instances %s has been successfully drained: %v", inst.InstanceID, result)
						stoptarget = append(stoptarget, inst.InstanceID)
						output, err := replacer.replaceUnusedInstance(asgname, stoptarget, dryrun)
						if err != nil {
							log.Fatalf("cannnot stop instance: %v", err)
						}
						log.Info.Printf("target ECS instances successfully stopped", output)
					}
				} else if inst.RunningTasks == 1 && inst.ImageID == imageid && inst.AvailabilityZone == az {
					log.Info.Printf("target ECS instances %s already runs newest AMI", inst.InstanceID)
				}
				wg.Done()
			}(inst)
		}
		wg.Wait()
	}

	return nil, nil
}
