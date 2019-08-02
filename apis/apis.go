package apis

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
)

//TargetAMI retains machine image name to delete.
type TargetAMI struct {
	ForceDeregister     bool
	ForceDeleteSnapshot bool
	Name                string
}

//Instance is am ec2api interface wrapper.
type Instance struct {
	ec2Api ec2iface.EC2API
}

//ImageSlice is slice of ec2images.
type ImageSlice []*ec2.Image

//VolumeSlice is slice of ec2volumes.
type VolumeSlice []*ec2.Snapshot

//NewEC2API creates new ec2api
func NewEC2API(session *session.Session, region string) ec2iface.EC2API {

	ec2 := ec2.New(session,
		&aws.Config{
			Region: aws.String(region),
		},
	)
	return ec2

}

//NewAutoScalingAPI creates new auto-scaling api
func NewAutoScalingAPI(session *session.Session, region string) autoscalingiface.AutoScalingAPI {

	as := autoscaling.New(session,
		&aws.Config{
			Region: aws.String(region),
		},
	)
	return as

}

//NewECSAPI creates new ecs api
func NewECSAPI(session *session.Session, region string) ecsiface.ECSAPI {

	ecssvc := ecs.New(session,
		&aws.Config{
			Region: aws.String(region),
		},
	)
	return ecssvc

}

// sort.Interface implementation for imageSlice
func (is ImageSlice) Len() int {
	return len(is)
}

func (is ImageSlice) Swap(i, j int) {
	is[i], is[j] = is[j], is[i]
}

func (is ImageSlice) Less(i, j int) bool {
	return *is[i].CreationDate > *is[j].CreationDate
}

func (is VolumeSlice) Len() int {
	return len(is)
}

func (is VolumeSlice) Swap(i, j int) {
	is[i], is[j] = is[j], is[i]
}

func (is VolumeSlice) Less(i, j int) bool {
	itime := *is[i].StartTime
	jtime := *is[j].StartTime
	return itime.After(jtime)
}
