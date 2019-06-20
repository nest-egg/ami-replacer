package actions

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/nest-egg/ami-replacer/log"

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
	ecsInstance, err := r.ecsInstance(c.Clustername, c.Asgname, newestimage, clusterSize)
	if err != nil {
		return nil, err
	}
	unusedInstances, err := r.unusedInstance(c.Clustername, newestimage)
	if err != nil {
		return nil, err
	}

	freeInstances, err := r.freeInstance(c.Clustername, c.Asgname, newestimage, clusterSize)
	if err != nil {
		return nil, err
	}

	clst := &cluster{
		name:            c.Clustername,
		ecsInstance:     ecsInstance,
		unusedInstances: unusedInstances,
		freeInstances:   freeInstances,
		size:            clusterSize,
		asg:             asg{c.Asgname, num, newestimage},
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
	log.Info.Printf("ECS instances %s has been successfully drained", inst.InstanceID)
	return result, nil
}
