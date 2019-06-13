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

//InfoAsg gets information of given auto-scaling groups.
func (replacer *Replacement) InfoAsg(asgname string) (grp *autoscaling.Group, err error) {

	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			aws.String(asgname),
		},
	}
	output, err := replacer.asg.AsgAPI.DescribeAutoScalingGroups(params)
	if err != nil {
		return nil, err
	}
	if len(output.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("There is not such autoscaling Group")
	}
	asgGroup := output.AutoScalingGroups[0]
	if len(asgGroup.Instances) == 0 {
		return nil, fmt.Errorf("AutoScaling Group is missing Instances: %+v", *asgGroup)
	}
	log.Debug.Println(asgGroup)
	return asgGroup, nil
}

//AmiAsg extracts imageID of current ASG from Launch Template.
func (replacer *Replacement) AmiAsg(instanceID string) (amiID string, err error) {

	params := &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}
	output, err := replacer.asg.AsgAPI.DescribeAutoScalingInstances(params)
	if err != nil {
		return "", fmt.Errorf("Error when execute DescribeAutoScalingInstances: %+v", err)
	}
	instance := output.AutoScalingInstances[0]
	if instance.LaunchTemplate == nil {
		return "", fmt.Errorf("AutoScaling Group is missing Instances: %+v", *instance)
	}
	launchtemplateid := instance.LaunchTemplate.LaunchTemplateId
	version := instance.LaunchTemplate.Version
	latest, err := replacer.asg.Ec2Api.DescribeLaunchTemplateVersions(&ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String(*launchtemplateid),
		Versions: []*string{
			aws.String(*version),
		},
	})
	if err != nil {
		return "", fmt.Errorf("Error when execute DescribeLaunchTemplateVersions: %+v", err)
	}
	amiid := *latest.LaunchTemplateVersions[0].LaunchTemplateData.ImageId
	return amiid, nil
}

//DeregisterAMI deregisters ami.
func (replacer *Replacement) DeregisterAMI(imageid string, owner string, imagename string, gen int, dryrun bool) (*ec2.DeregisterImageOutput, error) {
	imagesOutput, err := replacer.asg.Ec2Api.DescribeImages(&ec2.DescribeImagesInput{
		Owners: []*string{aws.String(owner)},
		Filters: []*ec2.Filter{{
			Name:   aws.String("name"),
			Values: []*string{aws.String(imagename)},
		}}})
	if err != nil {
		return nil, err
	}

	sort.Sort(apis.ImageSlice(imagesOutput.Images))
	len := apis.ImageSlice(imagesOutput.Images).Len()
	if len <= gen {
		return nil, fmt.Errorf("no outdated images")
	}
	images := make([]map[string]interface{}, 0, len)
	for i := gen - 1; i < len; i++ {

		if false {
			m := map[string]interface{}{
				"Name":         imagesOutput.Images[i].Name,
				"CreationDate": imagesOutput.Images[i].CreationDate,
				"ImageId":      imagesOutput.Images[i].ImageId,
			}
			images = append(images, m)
		}
		imageid := imagesOutput.Images[i].ImageId
		fmt.Println(imageid)
		deregisterOutput, err := replacer.asg.Ec2Api.DeregisterImage(&ec2.DeregisterImageInput{
			DryRun:  aws.Bool(dryrun),
			ImageId: aws.String(*imageid),
		})
		if err != nil {
			return deregisterOutput, err
		}
		log.Debug.Println(deregisterOutput)
	}
	return nil, nil
}
