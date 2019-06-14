package actions

import (
	"context"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/nest-egg/ami-replacer/fsm"
)

//Replacer is replacer interface.
type Replacer interface {
	ReplaceInstance(string, string, bool, string, string) (*autoscaling.Group, error)
	InfoAsg(string) (*autoscaling.Group, error)
	AmiAsg(string) (string, error)
	getEcsInstanceArn(string) ([]string, error)
	EcsInstanceStatus(string, []string) (*ecs.DescribeContainerInstancesOutput, error)
	replaceUnusedInstance(string, []string, bool) (*ec2.StopInstancesOutput, error)
	swapInstance(*cluster, string, bool) (*ec2.StopInstancesOutput, error)
	GetNewestAMI(string, string) (string, error)
	DeleteSnapshot(string, bool) (*ec2.DeleteSnapshotOutput, error)
	SearchUnusedSnapshot(string) (*ec2.DescribeSnapshotsOutput, error)
	VolumeExists(string) (*ec2.DescribeVolumesOutput, error)
	ImageExists(string) (*ec2.DescribeImagesOutput, error)
	getRegion(string) (string, error)
	DeregisterAMI(string, string, string, int, bool) (*ec2.DeregisterImageOutput, error)
}

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
	profile string) Replacer {

	asgroup := newAsg(region, profile)
	deploy := fsm.NewDeploy("start")
	return &Replacement{
		ctx:    ctx,
		asg:    asgroup,
		deploy: deploy,
	}
}
