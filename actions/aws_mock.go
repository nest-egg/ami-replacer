package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"github.com/nest-egg/ami-replacer/fsm"
)

type mockASGiface struct {
	autoscalingiface.AutoScalingAPI
}
type mockEC2iface struct {
	ec2iface.EC2API
}

type mockECSiface struct {
	ecsiface.ECSAPI
}

type mockAutoScaling struct {
	AsgAPI *mockASGiface
	Ec2Api *mockEC2iface
	EcsAPI *mockECSiface
}

//MockReplacement mocks Replacement.
type MockReplacement struct {
	ctx context.Context
	//api         *AutoScaling
	deploy      *fsm.Deploy
	asg         *mockAutoScaling
	asginstance *AsgInstance
}

func newMockAsg(region string, profile string) (asg *mockAutoScaling, err error) {

	return &mockAutoScaling{
		&mockASGiface{},
		&mockEC2iface{},
		&mockECSiface{},
	}, nil

}

//NewMockReplacer genetate new replacer object.
func NewMockReplacer(
	ctx context.Context,
	region string,
	profile string) *Replacement {

	asgroup := newAsg(region, profile)
	deploy := fsm.NewDeploy("start")
	asgroup.Ec2Api = &mockEC2iface{}
	asgroup.AsgAPI = &mockASGiface{}
	asgroup.EcsAPI = &mockECSiface{}
	return &Replacement{
		ctx:    ctx,
		asg:    asgroup,
		deploy: deploy,
	}
}

func (asg *mockASGiface) DescribeAutoScalingGroups(params *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {

	var output *autoscaling.DescribeAutoScalingGroupsOutput

	switch *params.AutoScalingGroupNames[0] {
	case "no_asg":
		output = &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{},
		}
	case "err_asg":
		return nil, fmt.Errorf("failed to describe asg")
	case "empty_asg":
		g := &autoscaling.Group{
			AutoScalingGroupName: aws.String("empty_asg"),
			AvailabilityZones: []*string{
				aws.String("ap-northeast-1a"),
				aws.String("ap-northeast-1c"),
			},
			DefaultCooldown:        aws.Int64(420),
			DesiredCapacity:        aws.Int64(3),
			HealthCheckGracePeriod: aws.Int64(300),
			HealthCheckType:        aws.String("EC2"),
			Instances:              []*autoscaling.Instance{},
			LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
				LaunchTemplateId:   aws.String("lt-00000000000000000"),
				LaunchTemplateName: aws.String("mytemplate"),
				Version:            aws.String("$Latest"),
			},
			MaxSize:                          aws.Int64(12),
			MinSize:                          aws.Int64(3),
			NewInstancesProtectedFromScaleIn: aws.Bool(false),
			TargetGroupARNs: []*string{
				aws.String("testarn"),
			},
			TerminationPolicies: []*string{
				aws.String("Default"),
			},
			VPCZoneIdentifier: aws.String("subnet-00000001"),
		}
		output = &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				g,
			},
		}
	default:
		createdTime := time.Now().UTC()
		instance := &autoscaling.Instance{
			AvailabilityZone: aws.String("ap-northeast-1c"),
			InstanceId:       aws.String("i-00000000000000000"),
			LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
				LaunchTemplateId:   aws.String("lt-00000000000000000"),
				LaunchTemplateName: aws.String("mytemplate"),
				Version:            aws.String("99"),
			},
			LifecycleState:       aws.String("InService"),
			ProtectedFromScaleIn: aws.Bool(false),
		}

		g := &autoscaling.Group{
			AutoScalingGroupName: aws.String("asg_ok"),
			AvailabilityZones: []*string{
				aws.String("ap-northeast-1a"),
				aws.String("ap-northeast-1c"),
			},
			CreatedTime:            &createdTime,
			DefaultCooldown:        aws.Int64(420),
			DesiredCapacity:        aws.Int64(3),
			HealthCheckGracePeriod: aws.Int64(300),
			HealthCheckType:        aws.String("EC2"),
			Instances: []*autoscaling.Instance{
				instance,
			},
			LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
				LaunchTemplateId:   aws.String("lt-00000000000000000"),
				LaunchTemplateName: aws.String("mytemplate"),
				Version:            aws.String("$Latest"),
			},
			MaxSize:                          aws.Int64(12),
			MinSize:                          aws.Int64(3),
			NewInstancesProtectedFromScaleIn: aws.Bool(false),
			TargetGroupARNs: []*string{
				aws.String("testarn"),
			},
			TerminationPolicies: []*string{
				aws.String("Default"),
			},
			VPCZoneIdentifier: aws.String("subnet-00000001"),
		}
		output = &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				g,
			},
		}
	}
	return output, nil
}

func (asg *mockASGiface) DescribeAutoScalingInstances(params *autoscaling.DescribeAutoScalingInstancesInput) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {

	var instances *autoscaling.DescribeAutoScalingInstancesOutput
	switch *params.InstanceIds[0] {
	case "exec_error":
		return nil, fmt.Errorf("failed to describe asg instances")
	case "no_launch_template":
		instances = &autoscaling.DescribeAutoScalingInstancesOutput{
			AutoScalingInstances: []*autoscaling.InstanceDetails{
				{
					LaunchTemplate: nil,
				},
			},
		}
	case "exec_error_2":
		instances = &autoscaling.DescribeAutoScalingInstancesOutput{
			AutoScalingInstances: []*autoscaling.InstanceDetails{
				{
					LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
						LaunchTemplateId: aws.String("error_id"),
						Version:          aws.String("$Latest"),
					},
				},
			},
		}
	case "instance-with-obsolete-image":
		instances = &autoscaling.DescribeAutoScalingInstancesOutput{
			AutoScalingInstances: []*autoscaling.InstanceDetails{
				{
					LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
						LaunchTemplateId: aws.String("obsolete"),
						Version:          aws.String("$Latest"),
					},
				},
			},
		}
	default:
		instances = &autoscaling.DescribeAutoScalingInstancesOutput{
			AutoScalingInstances: []*autoscaling.InstanceDetails{
				{
					LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
						LaunchTemplateId: aws.String("ok"),
						Version:          aws.String("$Latest"),
					},
				},
			},
		}
	}
	return instances, nil
}

func (ec *mockEC2iface) DescribeLaunchTemplateVersions(params *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {

	var output *ec2.DescribeLaunchTemplateVersionsOutput

	switch *params.LaunchTemplateId {
	case "error_id":
		return output, fmt.Errorf("failed to describr launch template versions")
	case "obsolete":
		version := &ec2.LaunchTemplateVersion{
			LaunchTemplateData: &ec2.ResponseLaunchTemplateData{
				ImageId: aws.String("ami-00000000000000002"),
			},
			LaunchTemplateId: aws.String("lt-00000000000000000"),
			VersionNumber:    aws.Int64(99),
		}
		output = &ec2.DescribeLaunchTemplateVersionsOutput{
			LaunchTemplateVersions: []*ec2.LaunchTemplateVersion{
				version,
			},
		}
	default:
		version := &ec2.LaunchTemplateVersion{
			LaunchTemplateData: &ec2.ResponseLaunchTemplateData{
				ImageId: aws.String("ami-00000000000000001"),
			},
			LaunchTemplateId: aws.String("lt-00000000000000000"),
			VersionNumber:    aws.Int64(99),
		}
		output = &ec2.DescribeLaunchTemplateVersionsOutput{
			LaunchTemplateVersions: []*ec2.LaunchTemplateVersion{
				version,
			},
		}
	}
	return output, nil
}

func (ec *mockEC2iface) DescribeImages(params *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {

	var output *ec2.DescribeImagesOutput
	switch *params.Filters[0].Values[0] {
	case "error*":
		return nil, fmt.Errorf("error executing DescribeImages")
	case "error2*":
		output = &ec2.DescribeImagesOutput{
			Images: []*ec2.Image{
				{
					CreationDate: aws.String("2019-01-01T00:00:01.000Z"),
					Name:         aws.String("testimage"),
					ImageId:      aws.String("ami-00000000000000001"),
				},
				{
					CreationDate: aws.String("2018-01-01T00:00:01.000Z"),
					Name:         aws.String("testimage2"),
					ImageId:      aws.String("ami-00000000000000002"),
				},
				{
					CreationDate: aws.String("2017-01-01T00:00:01.000Z"),
					Name:         aws.String("testimage2"),
					ImageId:      aws.String("error"),
				},
			},
		}
	default:
		output = &ec2.DescribeImagesOutput{
			Images: []*ec2.Image{
				{
					CreationDate: aws.String("2019-01-01T00:00:01.000Z"),
					Name:         aws.String("testimage"),
					ImageId:      aws.String("ami-00000000000000001"),
				},
				{
					CreationDate: aws.String("2018-01-01T00:00:01.000Z"),
					Name:         aws.String("testimage2"),
					ImageId:      aws.String("ami-00000000000000002"),
				},
				{
					CreationDate: aws.String("2017-01-01T00:00:01.000Z"),
					Name:         aws.String("testimage2"),
					ImageId:      aws.String("ami-00000000000000003"),
				},
			},
		}
	}
	return output, nil
}

func (ec *mockEC2iface) DeregisterImage(params *ec2.DeregisterImageInput) (*ec2.DeregisterImageOutput, error) {

	var output *ec2.DeregisterImageOutput
	switch *params.ImageId {
	case "error":
		return nil, fmt.Errorf("error executing DeregisterImage")
	default:
		output = &ec2.DeregisterImageOutput{}
	}
	return output, nil
}

func (ec *mockEC2iface) DeleteSnapshot(params *ec2.DeleteSnapshotInput) (*ec2.DeleteSnapshotOutput, error) {

	var output *ec2.DeleteSnapshotOutput
	switch *params.SnapshotId {
	case "error":
		return nil, fmt.Errorf("error executing DeleteSnapshot")
	default:
		output = &ec2.DeleteSnapshotOutput{}
	}
	return output, nil
}

func (ec *mockEC2iface) DescribeSnapshots(params *ec2.DescribeSnapshotsInput) (*ec2.DescribeSnapshotsOutput, error) {

	var output *ec2.DescribeSnapshotsOutput
	switch *params.OwnerIds[0] {
	case "error":
		return nil, fmt.Errorf("error executing DescribeSnapshot")
	default:
		output = &ec2.DescribeSnapshotsOutput{
			Snapshots: []*ec2.Snapshot{
				{
					OwnerId:    aws.String("owner"),
					SnapshotId: aws.String("snapshot1"),
					VolumeId:   aws.String("volume1"),
				},
				{
					OwnerId:    aws.String("owner"),
					SnapshotId: aws.String("snapshot2"),
					VolumeId:   aws.String("volume2"),
				},
			},
		}
	}
	return output, nil
}

func (ec *mockEC2iface) DescribeVolumes(params *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {

	var output *ec2.DescribeVolumesOutput
	switch *params.Filters[0].Values[0] {
	case "error":
		return nil, fmt.Errorf("error executing DescribeVolumes")
	default:
		output = &ec2.DescribeVolumesOutput{
			Volumes: []*ec2.Volume{
				{
					SnapshotId: aws.String("snapshot1"),
					VolumeId:   aws.String("volume1"),
				},
				{
					SnapshotId: aws.String("snapshot2"),
					VolumeId:   aws.String("volume2"),
				},
			},
		}
	}
	return output, nil
}

func (ec *mockEC2iface) DescribeInstances(params *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {

	var output *ec2.DescribeInstancesOutput
	switch *params.InstanceIds[0] {
	case "error":
		return nil, fmt.Errorf("failed to execute DescribeInstances")
	default:
		output = &ec2.DescribeInstancesOutput{
			Reservations: []*ec2.Reservation{
				{
					Instances: []*ec2.Instance{
						{
							Placement: &ec2.Placement{
								AvailabilityZone: aws.String("ap-northeast-1a"),
							},
							State: &ec2.InstanceState{
								Code: aws.Int64(80),
							},
						},
					},
				},
			},
		}
	}

	return output, nil
}

func (ec *mockEC2iface) StopInstances(params *ec2.StopInstancesInput) (*ec2.StopInstancesOutput, error) {
	var output *ec2.StopInstancesOutput
	output = &ec2.StopInstancesOutput{}
	return output, nil
}

func (ecsi *mockECSiface) ListContainerInstances(params *ecs.ListContainerInstancesInput) (*ecs.ListContainerInstancesOutput, error) {

	var output *ecs.ListContainerInstancesOutput
	switch *params.Cluster {
	case "error_cluster":
		return nil, fmt.Errorf("failed to execute ListContainerInstances")
	default:
		output = &ecs.ListContainerInstancesOutput{
			ContainerInstanceArns: []*string{
				aws.String("instance1"),
				aws.String("instance2"),
			},
		}
	}

	return output, nil
}

func (ecsi *mockECSiface) DescribeContainerInstances(params *ecs.DescribeContainerInstancesInput) (*ecs.DescribeContainerInstancesOutput, error) {

	var output *ecs.DescribeContainerInstancesOutput
	switch *params.Cluster {
	case "error-cluster2":
		return output, fmt.Errorf("failed to execute DescribeContainerInstances")
	case "1-running-tasks-and-empty-instance":
		output = &ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []*ecs.ContainerInstance{
				{
					Ec2InstanceId:        aws.String("instance1"),
					RunningTasksCount:    aws.Int64(1),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn1"),
				},
				{
					Ec2InstanceId:        aws.String("instance2"),
					RunningTasksCount:    aws.Int64(0),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn2"),
				},
			},
		}
	case "no-running-tasks":
		output = &ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []*ecs.ContainerInstance{
				{
					Ec2InstanceId:        aws.String("instance1"),
					RunningTasksCount:    aws.Int64(0),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn1"),
				},
				{
					Ec2InstanceId:        aws.String("instance2"),
					RunningTasksCount:    aws.Int64(0),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn2"),
				},
			},
		}
	case "no-empty-instances":
		output = &ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []*ecs.ContainerInstance{
				{
					Ec2InstanceId:        aws.String("instance1"),
					RunningTasksCount:    aws.Int64(1),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn1"),
				},
			},
		}
	case "rolling-deploy":
		output = &ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []*ecs.ContainerInstance{
				{
					Ec2InstanceId:        aws.String("instance1"),
					RunningTasksCount:    aws.Int64(1),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn1"),
				},
				{
					Ec2InstanceId:        aws.String("instance2"),
					RunningTasksCount:    aws.Int64(0),
					PendingTasksCount:    aws.Int64(1),
					ContainerInstanceArn: aws.String("arn2"),
				},
			},
		}
	case "during-deploy":
		output = &ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []*ecs.ContainerInstance{
				{
					Ec2InstanceId:        aws.String("instance1"),
					RunningTasksCount:    aws.Int64(0),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn1"),
				},
				{
					Ec2InstanceId:        aws.String("instance2"),
					RunningTasksCount:    aws.Int64(0),
					PendingTasksCount:    aws.Int64(1),
					ContainerInstanceArn: aws.String("arn2"),
				},
			},
		}
	default:
		output = &ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []*ecs.ContainerInstance{
				{
					Ec2InstanceId:        aws.String("instance1"),
					RunningTasksCount:    aws.Int64(1),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn1"),
				},
				{
					Ec2InstanceId:        aws.String("instance2"),
					RunningTasksCount:    aws.Int64(0),
					PendingTasksCount:    aws.Int64(0),
					ContainerInstanceArn: aws.String("arn2"),
				},
			},
		}

	}
	return output, nil
}

func (ecsi *mockECSiface) UpdateContainerInstancesState(*ecs.UpdateContainerInstancesStateInput) (*ecs.UpdateContainerInstancesStateOutput, error) {
	output := &ecs.UpdateContainerInstancesStateOutput{
		ContainerInstances: []*ecs.ContainerInstance{
			{
				Ec2InstanceId:        aws.String("instance1"),
				RunningTasksCount:    aws.Int64(1),
				PendingTasksCount:    aws.Int64(0),
				ContainerInstanceArn: aws.String("arn1"),
			},
			{
				Ec2InstanceId:        aws.String("instance2"),
				RunningTasksCount:    aws.Int64(0),
				PendingTasksCount:    aws.Int64(0),
				ContainerInstanceArn: aws.String("arn2"),
			},
		},
		Failures: []*ecs.Failure{
			{
				Arn:    aws.String("arn1"),
				Reason: aws.String("test failed."),
			},
		},
	}
	return output, nil
}
