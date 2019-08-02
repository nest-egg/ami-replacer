package main

import (
	"os"
	"testing"

	"github.com/nest-egg/ami-replacer/actions"
	"github.com/urfave/cli"
)

func TestMain(t *testing.T) {

	makeReplacer = actions.NewMockReplacer
	app := cli.NewApp()
	app.Name = "finbeeami"
	app.Usage = "remove unused ami"
	app.Version = "0.0.1"
	app.Commands = cmds
	args := os.Args[0:1]

	t.Run("remove images", func(t *testing.T) {
		//setup args
		args = append(args, "rmi")
		args = append(args, "--image", "test-infra*")
		args = append(args, "--owner", "owner")
		args = append(args, "--asgname", "myasg")

		err := app.Run(args)
		if err != nil {
			t.Errorf("got: %v\nwant: %v", err, nil)
		}
	})

	t.Run("replace", func(t *testing.T) {
		//setup args
		args = append(args, "replace")
		args = append(args, "--image", "test-infra*")
		args = append(args, "--owner", "owner")
		args = append(args, "--asgname", "myasg")

		err := app.Run(args)
		if err != nil {
			t.Errorf("got: %v\nwant: %v", err, nil)
		}
	})
}
