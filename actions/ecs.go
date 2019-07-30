package actions

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/nest-egg/ami-replacer/log"

	"github.com/cenkalti/backoff"
	"github.com/nest-egg/ami-replacer/config"
	"golang.org/x/xerrors"
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
	name      string
	size      int
	newestami string
}

func (r *Replacement) setClusterStatus(c *config.Config) (*cluster, error) {

	newestimage, err := r.newestAMI(c.Owner, c.Image)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get newest ami id: %w", err)
	}

	asginfo, err := r.asgInfo(c.Asgname)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get asg info: %w", err)
	}
	num := len(asginfo.Instances)
	clusterSize := len(asginfo.Instances)
	maxnum := int(*asginfo.MaxSize)
	if maxnum <= num+1 {
		return nil, xerrors.New("Max size of asg should be set to at least current size +1")
	}

	clst := &cluster{
		name: c.Clustername,
		size: clusterSize,
		asg:  asg{c.Asgname, num, newestimage},
	}

	ecsInstance, err := r.ecsInstance(clst)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get instances to replace: %w", err)
	}
	unusedInstances, err := r.unusedInstance(clst)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get unused instances with newest ami: %w", err)
	}

	freeInstances, err := r.freeInstance(clst)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get free instances: %w", err)
	}

	clst.ecsInstance = ecsInstance
	clst.unusedInstances = unusedInstances
	clst.freeInstances = freeInstances

	return clst, nil
}

func (r *Replacement) refreshClusterStatus(clst *cluster) (*cluster, error) {

	asginfo, err := r.asgInfo(clst.asg.name)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get asg info: %w", err)
	}
	clst.size = len(asginfo.Instances)
	clst.asg.size = len(asginfo.Instances)

	clst.ecsInstance, err = r.ecsInstance(clst)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get instances to replace: %w", err)
	}
	clst.unusedInstances, err = r.unusedInstance(clst)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get unused instances with newest ami: %w", err)
	}

	clst.freeInstances, err = r.freeInstance(clst)
	if err != nil {
		return nil, xerrors.Errorf("Failed to get free instances: %w", err)
	}

	return clst, nil
}

func (r *Replacement) clusterStatus(clustername string) (*ecs.DescribeContainerInstancesOutput, error) {
	arns, err := r.ecsInstanceArn(clustername)
	if err != nil {
		return nil, xerrors.Errorf("Cannnot get instance arn: %w", err)
	}
	status, err := r.ecsInstanceStatus(clustername, arns)
	if err != nil {
		return nil, xerrors.Errorf("Cannnot get ecs status : %w", err)
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
		return nil, xerrors.Errorf("Failed to list container instances: %w", err)
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
	output, err := r.asg.EcsAPI.DescribeContainerInstances(params)
	if err != nil {
		return nil, xerrors.Errorf("Failed to describe container instances: %w", err)
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
		return nil, xerrors.Errorf("Failed to update container instance state: %w", err)
	}

	b := newShortExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 50)

	counter := func() error {
		status, err := r.clusterStatus(inst.Cluster)
		if err != nil {
			return xerrors.Errorf("Failed to get cluster status: %w", err)
		}
		for _, st := range status.ContainerInstances {
			if *st.Status == "DRAINING" && *st.RunningTasksCount != int64(0) {
				return xerrors.New("Waiting for running new tasks")
			}
		}
		return nil
	}

	if err := backoff.Retry(counter, bf); err != nil {
		return nil, xerrors.New("Waiter has timed out")
	}

	log.Logger.Infof("ECS instances %s has been successfully drained", inst.InstanceID)
	return result, nil
}

func (r *Replacement) waitInstanceRunning(clst *cluster, num int) error {

	var count int
	b := newExponentialBackOff()
	bf := backoff.WithMaxRetries(b, 10)

	counter := func() error {
		status, err := r.clusterStatus(clst.name)
		if err != nil {
			return xerrors.Errorf("Failed to get cluster status: %w", err)
		}
		for _, st := range status.ContainerInstances {
			if *st.Status == "ACTIVE" {
				count++
			}
		}
		if count != num {
			return xerrors.New("Still waitng for instance running")
		}
		return nil
	}

	if err := backoff.Retry(counter, bf); err != nil {
		return xerrors.New("Waiter has timed out")
	}

	return nil
}
