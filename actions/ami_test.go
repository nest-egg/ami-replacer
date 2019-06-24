package actions

import (
	"context"
	"testing"

	"github.com/nest-egg/ami-replacer/config"
)

func TestAMI_InfoAsg(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name      string
		asgname   string
		shouldErr bool
	}{
		{
			name:    "ok",
			asgname: "asg_ok",
		},
		{
			name:      "no_asg",
			asgname:   "no_asg",
			shouldErr: true,
		},
		{
			name:      "error when describe asg",
			asgname:   "err_asg",
			shouldErr: true,
		},
		{
			name:      "zero_instance",
			asgname:   "empty_asg",
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockreplacer := NewMockReplacer(
				context.Background(),
				region,
				profile,
			)
			asginstance, err := mockreplacer.asgInfo(tc.asgname)
			_ = asginstance
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}

func TestAMI_Ami(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name       string
		instanceid string
		shouldErr  bool
	}{
		{
			name:       "ok",
			instanceid: "ok",
		},
		{
			name:       "exec error_DescribeAutoScalingInstances",
			instanceid: "exec_error",
			shouldErr:  true,
		},
		{
			name:       "no_launch_template",
			instanceid: "no_launch_template",
			shouldErr:  true,
		},
		{
			name:       "exec_error_DescribeLaunchTemplateVersions",
			instanceid: "exec_error_2",
			shouldErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockreplacer := NewMockReplacer(
				context.Background(),
				region,
				profile,
			)
			imageid, err := mockreplacer.Ami(tc.instanceid)
			_ = imageid
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}

func TestAMI_deregisterAmi(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name      string
		imageid   string
		owner     string
		image     string
		gen       int
		shouldErr bool
	}{
		{
			name:    "ok",
			imageid: "ok",
			owner:   "owner",
			image:   "ok*",
			gen:     2,
		},
		{
			name:      "exec_error_DescribeImages",
			imageid:   "exec_error_1",
			owner:     "owner",
			image:     "error*",
			gen:       2,
			shouldErr: true,
		},
		{
			name:      "exec_error_DeregisterImage",
			imageid:   "exec_error_2",
			owner:     "owner",
			image:     "error2*",
			gen:       2,
			shouldErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockreplacer := NewMockReplacer(
				context.Background(),
				region,
				profile,
			)
			conf := &config.Config{
				Image:      tc.image,
				Owner:      tc.owner,
				Generation: tc.gen,
				Dryrun:     false,
			}
			output, err := mockreplacer.deregisterAMI(conf)
			_ = output
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}
