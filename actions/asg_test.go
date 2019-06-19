package actions

import (
	"context"
	"testing"
	"github.com/nest-egg/ami-replacer/config"
)	

func TestASG_ReplaceInstance(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name        string
		asgname     string
		clustername string
		image       string
		owner       string
		shouldErr   bool
	}{
		{
			name:        "ok",
			asgname:     "ok",
			clustername: "test-cluster",
			image:       "testimage*",
			owner:       "owner",
		},
		{
			name:        "exec_error_getEcsInstanceArn",
			asgname:     "ok",
			clustername: "error-cluster",
			image:       "testimage*",
			owner:       "owner",
			shouldErr:   true,
		},
		{
			name:        "exec_error_EcsInstanceStatus",
			asgname:     "ok",
			clustername: "error-cluster2",
			image:       "testimage*",
			owner:       "owner",
			shouldErr:   true,
		},
		{
			name:        "1-running-tasks-and-empty-instance",
			asgname:     "ok",
			clustername: "1-running-tasks-and-empty-instance",
			image:       "ok*",
			owner:       "owner",
		},
		{
			name:        "no-running-tasks",
			asgname:     "ok",
			clustername: "no-running-tasks",
			image:       "ok*",
			owner:       "owner",
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
				Asgname:    tc.asgname,
				Image:      tc.image,
				Owner:      tc.owner,
				Clustername: tc.clustername,
				Dryrun:     false,
			}
			output, err := mockreplacer.ReplaceInstance(conf)
			_ = output
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}
