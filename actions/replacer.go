package actions

import (
	"context"
	"github.com/nest-egg/ami-replacer/fsm"
)

//Replacement defines replacement task.
type Replacement struct {
	ctx         context.Context
	deploy      *fsm.Deploy
	asg         *AutoScaling
	asginstance *AsgInstance
}

//AsgInstance retains status of each asg instance.
type AsgInstance struct {
	InstanceID       string
	InstanceArn      string
	ImageID          string
	RunningTasks     int
	PendingTasks     int
	Draining         bool
	ScalinProtection bool
	AvailabilityZone string
	Cluster          string
	ClusterSize      int
}

//NewReplacer genetate new replacer object.
func NewReplacer(
	ctx context.Context,
	region string,
	profile string) *Replacement {

	asgroup := newAsg(region, profile)
	deploy := fsm.NewDeploy("start")
	return &Replacement{
		ctx:    ctx,
		asg:    asgroup,
		deploy: deploy,
	}
}
