package actions

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/nest-egg/ami-replacer/log"

	"github.com/cenkalti/backoff"
	"github.com/nest-egg/ami-replacer/config"
)

type cluster struct {
	name            string
	ecsInstance     []AsgInstance
	unusedInstances []string
	freeInstances   []AsgInstance
	size            int
	asg             asg
}

type asg struct {
	name       string
	size       int
	newesetami string
}

func (r *Replacement) setClusterStatus(c *config.Config) (*cluster, error) {

	newestimage, err := r.newestAMI(c.Owner, c.Image)
	if err != nil {
		return nil, err
	}

	asginfo, err := r.asgInfo(c.Asgname)
	if err != nil {
		return nil, err
	}
	num := len(asginfo.Instances)
	if err != nil {
		return nil, err
	}
	clusterSize := len(asginfo.Instances)

	clst := &cluster{
		name: c.Clustername,
		size: clusterSize,
		asg:  asg{c.Asgname, num, newestimage},
	}

	ecsInstance, err := r.ecsInstance(clst)
	if err != nil {
		return nil, err
	}
	unusedInstances, err := r.unusedInstance(clst)
	if err != nil {
		return nil, err
	}

	freeInstances, err := r.freeInstance(clst)
	if err != nil {
		return nil, err
	}

	clst.ecsInstance = ecsInstance
	clst.unusedInstances = unusedInstances
	clst.freeInstances = freeInstances

	return clst, nil
}

func (r *Replacement) refreshClusterStatus(clst *cluster) (*cluster, error) {

	asginfo, err := r.asgInfo(clst.asg.name)
	if err != nil {
		return nil, err
	}
	clst.size = len(asginfo.Instances)

	clst.ecsInstance, err = r.ecsInstance(clst)
	if err != nil {
		return nil, err
	}
	clst.unusedInstances, err = r.unusedInstance(clst)
	if err != nil {
		return nil, err
	}

	clst.freeInstances, err = r.freeInstance(clst)
	if err != nil {
		return nil, err
	}

	return clst, nil
}

func (r *Replacement) clusterStatus(clustername string) (*ecs.DescribeContainerInstancesOutput, error) {
	arns, err := r.ecsInstanceArn(clustername)
	if err != nil {
		return nil, fmt.Errorf("cannnot get instance arn: %v", err)
	}
	status, err := r.ecsInstanceStatus(clustername, arns)
	if err != nil {
		return nil, fmt.Errorf("cannnot get ecs status : %v", err)
	}
	return status, nil
}

func (r *Replacement) ecsInstanceArn(clustername string) (out []string, err error) {
	var arns []string
	params := &ecs.ListContainerInstancesInput{
		Cluster: aws.String(clustername),
	}
	output, err := r.asg.EcsAPI.ListContainerInstances(params)
	if err != nil {
		return nil, err
	}
	for _, instance := range output.ContainerInstanceArns {
		arns = append(arns, aws.StringValue(instance))
	}
	return arns, err
}

func (r *Replacement) ecsInstanceStatus(clustername string, instances []string) (out *ecs.DescribeContainerInstancesOutput, err error) {

	params := &ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(clustername),
		ContainerInstances: aws.StringSlice(instances),
	}
	log.Debug.Println(params)
	output, err := r.asg.EcsAPI.DescribeContainerInstances(params)
	if err != nil {
		return nil, err
	}
	return output, err
}

func (r *Replacement) drainInstance(inst AsgInstance) (*ecs.UpdateContainerInstancesStateOutput, error) {

	params := &ecs.UpdateContainerInstancesStateInput{
		Cluster: aws.String(inst.Cluster),
		ContainerInstances: []*string{
			aws.String(inst.InstanceArn),
		},
		Status: aws.String("DRAINING"),
	}
	result, err := r.asg.EcsAPI.UpdateContainerInstancesState(params)
	if err != nil {
		return nil, err
	}

	b := newShortExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 50)

	counter := func() error {
		status, err := r.clusterStatus(inst.Cluster)
		if err != nil {
			return err
		}
		for _, st := range status.ContainerInstances {
			if *st.Status == "DRAINING" && *st.RunningTasksCount != int64(0) {
				return fmt.Errorf("Waiting for running new tasks")
			}
		}
		return nil
	}

	if err := backoff.Retry(counter, bf); err != nil {
		return nil, fmt.Errorf("waiter has timed out")
	}

	log.Info.Printf("ECS instances %s has been successfully drained", inst.InstanceID)
	return result, nil
}

func (r *Replacement) waitInstanceRunning(clst *cluster, num int) error {

	var count int
	b := newExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 10)

	counter := func() error {
		status, err := r.clusterStatus(clst.name)
		if err != nil {
			return err
		}
		for _, st := range status.ContainerInstances {
			if *st.Status == "ACTIVE" {
				count++
			}
		}
		if count != num {
			return fmt.Errorf("still waitng for instance running")
		}
		return nil
	}

	if err := backoff.Retry(counter, bf); err != nil {
		return fmt.Errorf("waiter has timed out")
	}

	return nil
}
