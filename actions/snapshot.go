package actions

import (
	//"fmt"
	//"regexp"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/nest-egg/ami-replacer/apis"
)

//NewestAMI returns newest AMI.
func (r *Replacement) NewestAMI(owner string, image string) (imageid string, err error) {

	imagesOutput, err := r.asg.Ec2Api.DescribeImages(&ec2.DescribeImagesInput{
		Owners: []*string{aws.String(owner)},
		Filters: []*ec2.Filter{{
			Name:   aws.String("name"),
			Values: []*string{aws.String(image)},
		}}})
	if err != nil {
		return "", err
	}

	sort.Sort(apis.ImageSlice(imagesOutput.Images))
	newestimageid := imagesOutput.Images[0].ImageId
	return *newestimageid, nil
}

//DeleteSnapshot deletes snapshot of given id.
func (r *Replacement) DeleteSnapshot(snapshotid string, dryrun bool) (result *ec2.DeleteSnapshotOutput, err error) {

	params := &ec2.DeleteSnapshotInput{
		DryRun:     aws.Bool(dryrun),
		SnapshotId: aws.String(snapshotid),
	}
	output, err := r.asg.Ec2Api.DeleteSnapshot(params)
	if err != nil {
		return nil, err
	}
	return output, nil
}

//SearchUnusedSnapshot finds snapshot not used by any volumes.
func (r *Replacement) SearchUnusedSnapshot(ownerid string) (result *ec2.DescribeSnapshotsOutput, err error) {

	params := &ec2.DescribeSnapshotsInput{
		OwnerIds: []*string{
			aws.String(ownerid),
		},
	}
	output, err := r.asg.Ec2Api.DescribeSnapshots(params)
	if err != nil {
		return nil, err
	}
	return output, nil
}

//VolumeExists find if volume exists for given snapshot id.
func (r *Replacement) VolumeExists(snapshotid string) (result *ec2.DescribeVolumesOutput, err error) {

	params := &ec2.DescribeVolumesInput{

		Filters: []*ec2.Filter{{
			Name: aws.String("snapshot-id"),
			Values: []*string{
				aws.String(snapshotid),
			},
		}}}
	output, err := r.asg.Ec2Api.DescribeVolumes(params)
	if err != nil {
		return nil, err
	}
	if len(output.Volumes) == 0 {
		return nil, nil
	}
	return output, nil
}

//ImageExists finds existing images for given snapshot id.
func (r *Replacement) ImageExists(snapshotid string) (result *ec2.DescribeImagesOutput, err error) {

	params := &ec2.DescribeImagesInput{

		Filters: []*ec2.Filter{{
			Name: aws.String("block-device-mapping.snapshot-id"),
			Values: []*string{
				aws.String(snapshotid),
			},
		}}}
	output, err := r.asg.Ec2Api.DescribeImages(params)
	if err != nil {
		return nil, err
	}
	if len(output.Images) == 0 {
		return nil, nil
	}
	return output, nil
}
