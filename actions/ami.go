package actions

import (
	"fmt"
	"sort"

	"github.com/nest-egg/ami-replacer/apis"
	"github.com/nest-egg/ami-replacer/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

//InfoAsg gets information of auto-scaling groups.
func (r *Replacement) asgInfo(asgname string) (grp *autoscaling.Group, err error) {

	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			aws.String(asgname),
		},
	}
	output, err := r.asg.AsgAPI.DescribeAutoScalingGroups(params)
	if err != nil {
		return nil, err
	}
	log.Debug.Printf("described asg... %v", output)
	if len(output.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("There is not such autoscaling Group as %s", asgname)
	}
	asgGroup := output.AutoScalingGroups[0]
	if len(asgGroup.Instances) == 0 {
		return nil, fmt.Errorf("missing Instances: %+v", *asgGroup)
	}
	log.Debug.Println(asgGroup)
	return asgGroup, nil
}

//Ami extracts imageID of current ASG from Launch Template.
func (r *Replacement) Ami(instanceID string) (amiID string, err error) {

	params := &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}
	output, err := r.asg.AsgAPI.DescribeAutoScalingInstances(params)
	if err != nil {
		return "", err
	}
	instance := output.AutoScalingInstances[0]
	if instance.LaunchTemplate == nil {
		return "", fmt.Errorf("AutoScaling Group is missing Instances: %+v", *instance)
	}
	launchtemplateid := instance.LaunchTemplate.LaunchTemplateId
	version := instance.LaunchTemplate.Version
	latest, err := r.asg.Ec2Api.DescribeLaunchTemplateVersions(&ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String(*launchtemplateid),
		Versions: []*string{
			aws.String(*version),
		},
	})
	if err != nil {
		return "", err
	}
	amiid := *latest.LaunchTemplateVersions[0].LaunchTemplateData.ImageId
	return amiid, nil
}

//DeregisterAMI deregisters ami.
func (r *Replacement) DeregisterAMI(imageid string, owner string, imagename string, gen int, dryrun bool) (*ec2.DeregisterImageOutput, error) {
	i, err := r.asg.Ec2Api.DescribeImages(&ec2.DescribeImagesInput{
		Owners: []*string{aws.String(owner)},
		Filters: []*ec2.Filter{{
			Name:   aws.String("name"),
			Values: []*string{aws.String(imagename)},
		}}})
	if err != nil {
		return nil, err
	}

	sort.Sort(apis.ImageSlice(i.Images))
	len := apis.ImageSlice(i.Images).Len()
	if len <= gen {
		return nil, fmt.Errorf("no outdated images")
	}
	images := make([]map[string]interface{}, 0, len)
	for j := gen - 1; j < len; j++ {

		if false {
			m := map[string]interface{}{
				"Name":         i.Images[j].Name,
				"CreationDate": i.Images[j].CreationDate,
				"ImageId":      i.Images[j].ImageId,
			}
			images = append(images, m)
		}
		imageid := i.Images[j].ImageId
		_, err := r.asg.Ec2Api.DeregisterImage(&ec2.DeregisterImageInput{
			DryRun:  aws.Bool(dryrun),
			ImageId: aws.String(*imageid),
		})
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}
