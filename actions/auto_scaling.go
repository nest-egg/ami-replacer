package actions

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"github.com/nest-egg/ami-replacer/apis"
)

// AutoScaling has asg and ec2 api interfaces.
type AutoScaling struct {
	AsgAPI autoscalingiface.AutoScalingAPI
	Ec2Api ec2iface.EC2API
	EcsAPI ecsiface.ECSAPI
}

func newAsg(region string, profile string) (asg *AutoScaling) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Profile:           profile,
	}))

	ec2Api := apis.NewEC2API(
		sess,
		region,
	)
	asgAPI := apis.NewAutoScalingAPI(
		sess,
		region,
	)
	ecsAPI := apis.NewECSAPI(
		sess,
		region,
	)

	return &AutoScaling{
		AsgAPI: asgAPI,
		Ec2Api: ec2Api,
		EcsAPI: ecsAPI,
	}

}
