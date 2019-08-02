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
	cli.VersionFlag = cli.BoolFlag{Name: "version, V"}
	rmiFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run, d",
			Usage: "dry run",
		},
		cli.StringFlag{
			Name:  "image, i",
			Value: "other",
			Usage: "image name",
		},
		cli.StringFlag{
			Name:  "owner, o",
			Value: "admin",
			Usage: "owner of amis",
		},
		cli.IntFlag{
			Name:  "gen,g",
			Value: 2,
			Usage: "max generations to retain",
		},
		cli.BoolFlag{
			Name:  "verbose,v",
			Usage: "enable debug mode",
		},
	}

	rmsFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run, d",
			Usage: "dry run",
		},
		cli.StringFlag{
			Name:  "owner",
			Value: "admin",
			Usage: "owner of amis",
		},
		cli.BoolFlag{
			Name:  "verbose,v",
			Usage: "enable debug mode",
		},
	}

	rplFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run, d",
			Usage: "dry run",
		},
		cli.StringFlag{
			Name:  "asgname,a",
			Value: "asg",
			Usage: "auto scaling group name",
		},
		cli.StringFlag{
			Name:  "clustername,c",
			Value: "admin",
			Usage: "ecs cluster name",
		},
		cli.StringFlag{
			Name:  "image,i",
			Value: "other",
			Usage: "image name",
		},
		cli.StringFlag{
			Name:  "owner,o",
			Value: "admin",
			Usage: "owner of amis",
		},
		cli.BoolFlag{
			Name:  "verbose,v",
			Usage: "enable debug mode",
		},
	}

	cmds = []cli.Command{
		{
			Name:    "rmi",
			Aliases: []string{"delete-images"},
			Usage:   "delete AMIs",
			Flags:   rmiFlags,
			Action:  removeAMIs,
		},
		{
			Name:    "rms",
			Aliases: []string{"remove-snapshots"},
			Usage:   "remove unused Snapshots",
			Flags:   rmsFlags,
			Action:  removeSnapshots,
		},
		{
			Name:    "rpl",
			Aliases: []string{"replace"},
			Usage:   "replace asg instance",
			Flags:   rplFlags,
			Action:  replaceInstances,
		},
	}
	region = "ap-northeast-1"
	profile = "admin"
}

func main() {
	doMain(os.Args)
}

func doMain(args []string) {
	app := cli.NewApp()
	app.Name = "ami-replacer"
	app.Usage = "replace ecs instance and amis"
	app.Version = "0.1"
	app.Commands = cmds
	app.Action = noArgs

	err := app.Run(args)
	if err != nil {
		log.Logger.Fatalf("Failed to run cmd: %+v", err)
	}
}

func noArgs(context *cli.Context) error {

	cli.ShowAppHelp(context)
	return cli.NewExitError("No args provided", 2)
}

func removeAMIs(ctx *cli.Context) error {
	conf := config.SetConfig(ctx)
	log.InitLogger(conf.Debug)

	log.Logger.Infof("AMI prefix to delete: %s\n", conf.Image)

	_, err := config.ParseRegion(region)
	if err != nil {
		return xerrors.Errorf("Invalid aws region: %w", err)
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
		return xerrors.Errorf("Failed to remove AMIs: %w", err)
	}
	log.Logger.Info("Successfully removed all unused AMIs")
	return nil
}

func removeSnapshots(ctx *cli.Context) error {
	conf := config.SetConfig(ctx)
	log.InitLogger(conf.Debug)

	r := makeReplacer(
		context.Background(),
		region,
		profile,
	)

	err := r.RemoveSnapShots(conf)
	if err != nil {
		return xerrors.Errorf("Failed to remove snapshots: %w", err)
	}
	log.Logger.Info("Successfully removed all unused snapshots")
	return nil
}

func replaceInstances(ctx *cli.Context) error {
	conf := config.SetConfig(ctx)
	log.InitLogger(conf.Debug)

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
		return xerrors.Errorf("Failed to replace instance: %w", err)
	}
	return nil
}
