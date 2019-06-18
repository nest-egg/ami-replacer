package actions

import (
	"fmt"
	
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/nest-egg/ami-replacer/log"
)


type cluster struct {
	Clustername     string
	EcsInstance     []AsgInstance
	UnusedInstances []string
	FreeInstances   []AsgInstance
	ClusterSize     int
}

func (replacer *Replacement) setClusterStatus(asgname string, clustername string, newestimage string) (*cluster, error) {
	asgGroup, err := replacer.InfoAsg(asgname)
	if err != nil {
		return nil, err
	}
	clusterSize := len(asgGroup.Instances)
	ecsInstance, err := replacer.getECSInstance(clustername, asgname, newestimage, clusterSize)
	if err != nil {
		return nil, err
	}
	unusedInstances, err := replacer.getUnusedInstance(clustername, newestimage)
	if err != nil {
		return nil, err
	}

	freeInstances, err := replacer.getFreeInstance(clustername, asgname, newestimage, clusterSize)
	if err != nil {
		return nil, err
	}

	clst := &cluster{
		Clustername:     clustername,
		EcsInstance:     ecsInstance,
		UnusedInstances: unusedInstances,
		FreeInstances:   freeInstances,
		ClusterSize:     clusterSize,
	}
	return clst, nil
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
	log.Info.Printf("ECS instances %s has been successfully drained", inst.InstanceID)
	return result, nil
}