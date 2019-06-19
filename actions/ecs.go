package actions

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/nest-egg/ami-replacer/log"
)

//Cluster represents ecs cluster with instances to replace.
type Cluster struct {
	// Clustername is ecs cluster name
	Clustername string

	// ECSInstance is instances to replace
	EcsInstance []AsgInstance

	// UnusedInstances is the lists of the ids of empty instance wich is running on old amis.
	// empty instance means each instance has no running or pending ecs tasks.
	UnusedInstances []string

	// FreeInstances is the lists of AsgInstance of empty instance wich is running on newest amis.
	FreeInstances []AsgInstance

	// ClusterSize is ecs cluster size
	ClusterSize int
}

func (r *Replacement) setClusterStatus(asgname string, clustername string, newestimage string) (*Cluster, error) {
	asgGroup, err := r.asgInfo(asgname)
	if err != nil {
		return nil, err
	}
	clusterSize := len(asgGroup.Instances)
	ecsInstance, err := r.ecsInstance(clustername, asgname, newestimage, clusterSize)
	if err != nil {
		return nil, err
	}
	unusedInstances, err := r.unusedInstance(clustername, newestimage)
	if err != nil {
		return nil, err
	}

	freeInstances, err := r.freeInstance(clustername, asgname, newestimage, clusterSize)
	if err != nil {
		return nil, err
	}

	clst := &Cluster{
		Clustername:     clustername,
		EcsInstance:     ecsInstance,
		UnusedInstances: unusedInstances,
		FreeInstances:   freeInstances,
		ClusterSize:     clusterSize,
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
