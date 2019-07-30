package actions

import (
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/nest-egg/ami-replacer/apis"
	"golang.org/x/xerrors"
)

func (r *Replacer) newestAMI(owner string, image string) (imageid string, err error) {

	output, err := r.asg.Ec2Api.DescribeImages(&ec2.DescribeImagesInput{
		Owners: []*string{aws.String(owner)},
		Filters: []*ec2.Filter{{
			Name:   aws.String("name"),
			Values: []*string{aws.String(image)},
		}}})
	if err != nil {
		return "", xerrors.Errorf("Failed to describe images: %w", err)
	}

	sort.Sort(apis.ImageSlice(output.Images))
	newestimageid := output.Images[0].ImageId
	return *newestimageid, nil
}

func (r *Replacer) deleteSnapshot(snapshotid string) (result *ec2.DeleteSnapshotOutput, err error) {

	params := &ec2.DeleteSnapshotInput{
		DryRun:     aws.Bool(dryrun),
		SnapshotId: aws.String(snapshotid),
	}
	output, err := r.asg.Ec2Api.DeleteSnapshot(params)
	if err != nil {
		return nil, xerrors.Errorf("Failed to delete snapshot: %w", err)
	}
	return output, nil
}

func (r *Replacer) searchUnusedSnapshot(ownerid string) (result *ec2.DescribeSnapshotsOutput, err error) {

	params := &ec2.DescribeSnapshotsInput{
		OwnerIds: []*string{
			aws.String(ownerid),
		},
	}
	output, err := r.asg.Ec2Api.DescribeSnapshots(params)
	if err != nil {
		return nil, xerrors.Errorf("Failed to describe snapshots: %w", err)
	}
	return output, nil
}

func (r *Replacer) volumeExists(snapshotid string) (result *ec2.DescribeVolumesOutput, err error) {

	params := &ec2.DescribeVolumesInput{

		Filters: []*ec2.Filter{{
			Name: aws.String("snapshot-id"),
			Values: []*string{
				aws.String(snapshotid),
			},
		}}}
	output, err := r.asg.Ec2Api.DescribeVolumes(params)
	if err != nil {
		return nil, xerrors.Errorf("Failed to describe volumes: %w", err)
	}
	return output, nil
}

func (r *Replacer) imageExists(snapshotid string) (result *ec2.DescribeImagesOutput, err error) {

	params := &ec2.DescribeImagesInput{

		Filters: []*ec2.Filter{{
			Name: aws.String("block-device-mapping.snapshot-id"),
			Values: []*string{
				aws.String(snapshotid),
			},
		}}}
	output, err := r.asg.Ec2Api.DescribeImages(params)
	if err != nil {
		return nil, xerrors.Errorf("Failed to describe images: %w", err)
	}
	return output, nil
}
