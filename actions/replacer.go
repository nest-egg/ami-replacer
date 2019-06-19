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
	InstanceID       string `json:"instanceID"`
	InstanceArn      string `json:"instanceArn"`
	ImageID          string `json:"imageID"`
	RunningTasks     int    `json:"runningtasks"`
	PendingTasks     int    `json:"pendingtasks"`
	Draining         bool   `json:"draining"`
	ScalinProtection bool   `json:"scaleinprotection"`
	AvailabilityZone string `json:"availabilityzone"`
	DeleteFlag       bool   `json:"deleteflag"`
	Cluster          string `json:"cluster"`
	ClusterSize      int    `json:"clustersize"`
	Asgname          string `json:"asgname"`
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
