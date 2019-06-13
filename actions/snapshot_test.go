package actions

import (
	"context"
	"testing"
)

func TestSnapshot_GetNewestAMI(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name      string
		owner     string
		image     string
		shouldErr bool
	}{
		{
			name:  "ok",
			owner: "owner",
			image: "testimage*",
		},
		{
			name:      "error",
			owner:     "owner",
			image:     "error*",
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
			output, err := mockreplacer.GetNewestAMI(tc.owner, tc.image)
			_ = output
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}

func TestSnapshot_DeleteSnapshot(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name       string
		snapshotid string
		shouldErr  bool
	}{
		{
			name:       "ok",
			snapshotid: "ok",
		},
		{
			name:       "error",
			snapshotid: "error",
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
			output, err := mockreplacer.DeleteSnapshot(tc.snapshotid, false)
			_ = output
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}

func TestSnapshot_SearchUnusedSnapshot(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name      string
		ownerid   string
		shouldErr bool
	}{
		{
			name:    "ok",
			ownerid: "ok",
		},
		{
			name:      "error",
			ownerid:   "error",
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
			output, err := mockreplacer.SearchUnusedSnapshot(tc.ownerid)
			_ = output
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}

func TestSnapshot_VolumeExists(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name       string
		snapshotid string
		shouldErr  bool
	}{
		{
			name:       "ok",
			snapshotid: "ok",
		},
		{
			name:       "error",
			snapshotid: "error",
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
			output, err := mockreplacer.VolumeExists(tc.snapshotid)
			_ = output
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}

func TestSnapshot_ImageExists(t *testing.T) {
	region := "ap-northeast-1"
	profile := "admin"
	testCases := []struct {
		name       string
		snapshotid string
		shouldErr  bool
	}{
		{
			name:       "ok",
			snapshotid: "ok",
		},
		{
			name:       "error",
			snapshotid: "error*",
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
			output, err := mockreplacer.ImageExists(tc.snapshotid)
			_ = output
			if err == nil && tc.shouldErr {
				t.Errorf("should raise error: %v", err)
			}

		})

	}
}
