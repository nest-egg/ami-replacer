package actions

import (
	"context"
	"github.com/nest-egg/ami-replacer/fsm"
)

//Replacer defines replacement task.
type Replacer struct {
	ctx      context.Context
	deploy   *fsm.Deploy
	asg      *AutoScaling
	instance *Instance
}

//Instance retains status of each asg instance.
type Instance struct {
	InstanceID   string
	InstanceArn  string
	ImageID      string
	RunningTasks int
	PendingTasks int
	Draining     bool
	Cluster      string
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
