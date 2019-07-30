package actions

import (
	"context"
	"github.com/nest-egg/ami-replacer/fsm"
)

//Replacer defines replacement task.
type Replacer struct {
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
}

//NewReplacer genetate new replacer object.
func NewReplacer(
	ctx context.Context,
	region string,
	profile string) *Replacer {

	asgroup := newAsg(region, profile)
	deploy := fsm.NewDeploy("start")
	return &Replacer{
		ctx:    ctx,
		asg:    asgroup,
		deploy: deploy,
	}
}
