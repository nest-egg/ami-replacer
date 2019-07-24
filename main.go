package main

import (
	"context"
	"os"

	"github.com/urfave/cli"

	"github.com/nest-egg/ami-replacer/actions"
	"github.com/nest-egg/ami-replacer/config"
	"github.com/nest-egg/ami-replacer/log"
	"golang.org/x/xerrors"
)

var (
	cmds     []cli.Command
	rmiFlags []cli.Flag
	rmsFlags []cli.Flag
	rplFlags []cli.Flag
	asg      actions.AutoScaling
	region   string
	profile  string
	owner    string
	image    string
	dryrun   bool
)

var makeReplacer = actions.NewReplacer

func init() {
	rmiFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run, d",
			Usage: "do not actually perform any operation.",
		},
		cli.StringFlag{
			Name:  "image",
			Value: "other",
			Usage: "image name",
		},
		cli.StringFlag{
			Name:  "owner",
			Value: "admin",
			Usage: "ami owner",
		},
		cli.IntFlag{
			Name:  "gen",
			Value: 2,
			Usage: "max gen",
		},
		cli.BoolFlag{
			Name:  "verbose,v",
			Usage: "enable debug mode",
		},
	}

	rmsFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run, d",
			Usage: "do not actually perform any operation.",
		},
		cli.StringFlag{
			Name:  "owner",
			Value: "admin",
			Usage: "ami owner",
		},
		cli.BoolFlag{
			Name:  "verbose,v",
			Usage: "enable debug mode",
		},
	}

	rplFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run, d",
			Usage: "do not actually perform any operation.",
		},
		cli.StringFlag{
			Name:  "asgname",
			Value: "asg",
			Usage: "autoscaling group name",
		},
		cli.StringFlag{
			Name:  "clustername",
			Value: "admin",
			Usage: "ecs cluster name",
		},
		cli.StringFlag{
			Name:  "image",
			Value: "other",
			Usage: "image name",
		},
		cli.StringFlag{
			Name:  "owner",
			Value: "admin",
			Usage: "ami owner",
		},
		cli.BoolFlag{
			Name:  "verbose,v",
			Usage: "enable debug mode",
		},
	}

	cmds = []cli.Command{
		{
			Name:    "rmi",
			Aliases: []string{"delete", "d"},
			Usage:   "delete AMIs",
			Flags:   rmiFlags,
			Action:  removeAMIs,
		},
		{
			Name:    "rms",
			Aliases: []string{"delete-snap", "ds"},
			Usage:   "remove unused Snapshots",
			Flags:   rmsFlags,
			Action:  removeSnapshots,
		},
		{
			Name:    "asg",
			Aliases: []string{"replace", "r"},
			Usage:   "replace asg instance",
			Flags:   rplFlags,
			Action:  replaceInstances,
		},
	}
	region = "ap-northeast-1"
	profile = "admin"
	log.SetLevel("info")
}

func main() {
	doMain(os.Args)
}

func doMain(args []string) {
	app := cli.NewApp()
	app.Name = "finbeeami"
	app.Usage = "remove unused ami"
	app.Version = "0.0.1"
	app.Commands = cmds
	app.Action = noArgs

	err := app.Run(args)
	if err != nil {
		log.Fatalf("failed to excute cmd: %+v", err)
	}
}

func noArgs(context *cli.Context) error {

	cli.ShowAppHelp(context)
	return cli.NewExitError("no commands provided", 2)
}

func removeAMIs(ctx *cli.Context) error {
	conf := config.SetConfig(ctx)
	if conf.Debug {
		log.SetLevel("debug")
	}

	log.Info.Printf("ami prefix to delete       : %+v\n", conf.Image)

	_, err := config.ParseRegion(region)
	if err != nil {
		return xerrors.Errorf("aws region is invalid!: %w", err)
	}

	if !config.IsValidProfile(profile) {
		return xerrors.New("Invalid config")
	}

	r := makeReplacer(
		context.Background(),
		region,
		profile,
	)

	if err := r.RemoveAMIs(conf); err != nil {
		return xerrors.Errorf("failed to remove AMIs: %w", err)
	}
	log.Info.Println("Successfully removed all unused AMIs")
	return nil
}

func removeSnapshots(ctx *cli.Context) error {
	conf := config.SetConfig(ctx)
	if conf.Debug {
		log.SetLevel("debug")
	}
	r := makeReplacer(
		context.Background(),
		region,
		profile,
	)

	err := r.RemoveSnapShots(conf)
	if err != nil {
		return xerrors.Errorf("failed to remove snapshots: %w", err)
	}
	log.Info.Println("Successfully removed all unused snapshots")
	return nil
}

func replaceInstances(ctx *cli.Context) error {
	conf := config.SetConfig(ctx)
	if conf.Debug {
		log.SetLevel("debug")
	}

	_, err := config.ParseRegion(region)
	if err != nil {
		return xerrors.Errorf("aws region is invalid!: %w", err)
	}

	if !config.IsValidProfile(profile) {
		return xerrors.New("Invalid Config")
	}

	r := makeReplacer(
		context.Background(),
		region,
		profile,
	)

	if err := r.ReplaceInstance(conf); err != nil {
		return xerrors.Errorf("failed to replace instance: %w", err)
	}
	return nil
}
