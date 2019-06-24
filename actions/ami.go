package actions

import (
	"fmt"
	"sort"

	"github.com/nest-egg/ami-replacer/apis"
	"github.com/nest-egg/ami-replacer/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/nest-egg/ami-replacer/config"
)

//Ami extracts imageID of current ASG from Launch Template.
func (r *Replacement) Ami(id string) (string, error) {

	params := &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: []*string{
			aws.String(id),
		},
	}
	output, err := r.asg.AsgAPI.DescribeAutoScalingInstances(params)
	if err != nil {
		return "", err
	}

	inst := output.AutoScalingInstances[0]
	if inst.LaunchTemplate == nil {
		return "", fmt.Errorf("AutoScaling Group is missing Instances: %+v", *inst)
	}

	templateID := inst.LaunchTemplate.LaunchTemplateId
	ver := inst.LaunchTemplate.Version
	latest, err := r.asg.Ec2Api.DescribeLaunchTemplateVersions(&ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String(*templateID),
		Versions: []*string{
			aws.String(*ver),
		},
	})
	if err != nil {
		return "", err
	}

	amiid := *latest.LaunchTemplateVersions[0].LaunchTemplateData.ImageId
	return amiid, nil
}

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

func (r *Replacement) deregisterAMI(c *config.Config) (*ec2.DeregisterImageOutput, error) {
	gen := c.Generation

	params := &ec2.DescribeImagesInput{
		Owners: []*string{aws.String(c.Owner)},
		Filters: []*ec2.Filter{{
			Name:   aws.String("name"),
			Values: []*string{aws.String(c.Image)},
		}}}
	i, err := r.asg.Ec2Api.DescribeImages(params)
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
		log.Info.Printf("images to delete: %v", *imageid)
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
