package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/urfave/cli"

	"github.com/nest-egg/ami-replacer/actions"
	"github.com/nest-egg/ami-replacer/apis"
	"github.com/nest-egg/ami-replacer/config"
	"github.com/nest-egg/ami-replacer/log"
)

var (
	cmds         []cli.Command
	rmiFlags     []cli.Flag
	rmsFlags     []cli.Flag
	replaceFlags []cli.Flag
	asg          actions.AutoScaling
	region       string
	profile      string
	owner        string
	image        string
	dryrun       bool
)

var makeReplacer = actions.NewReplacer

func init() {
	rmiFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run, d",
			Usage: "do not actually perform any operation.",
		},
		cli.BoolFlag{
			Name:  "delete-snapshot, r",
			Usage: "(default: false)",
		},
		cli.StringFlag{
			Name:  "asgname",
			Value: "asg",
			Usage: "autoscaling group name",
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
			Name:  "test,t",
			Usage: "enable test",
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
			Name:  "test,t",
			Usage: "enable test",
		},
		cli.BoolFlag{
			Name:  "verbose,v",
			Usage: "enable debug mode",
		},
	}

	replaceFlags = []cli.Flag{
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
			Name:  "test,t",
			Usage: "enable test",
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
			Flags:   replaceFlags,
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
		log.Fatal(err)
	}
}

func noArgs(context *cli.Context) error {

	cli.ShowAppHelp(context)
	return cli.NewExitError("no commands provided", 2)
}

func removeAMIs(clicontext *cli.Context) error {
	conf := &config.Config{
		Asgname:    clicontext.String("asgname"),
		Image:      clicontext.String("image"),
		Owner:      clicontext.String("owner"),
		Dryrun:     clicontext.Bool("dry-run"),
		Debug:      clicontext.Bool("verbose"),
		Generation: clicontext.Int("gen"),
	}

	if conf.Debug {
		log.SetLevel("debug")
	}
	log.Info.Printf("ami prefix to delete       : %+v\n", conf.Image)

	_, err := config.ParseRegion(region)
	if err != nil {
		log.Fatalf("aws region is invalid! %v", err)
	}

	if !config.IsValidProfile(profile) {
		log.Fatal("Invalid config")
	}

	replacer := makeReplacer(
		context.Background(),
		region,
		profile,
	)

	asginstance, err := replacer.InfoAsg(conf.Asgname)
	instanceid := asginstance.Instances[0].InstanceId
	imageid, err := replacer.AmiAsg(*instanceid)
	log.Debug.Println(imageid)

	output, err := replacer.DeregisterAMI(imageid, conf.Owner, conf.Image, conf.Generation, conf.Dryrun)
	if err != nil {
		log.Fatalf("deregister failed! %v", err)
	}

	log.Debug.Println(output)
	return err
}

func removeSnapshots(clicontext *cli.Context) error {

	conf := &config.Config{
		Owner:  clicontext.String("owner"),
		Dryrun: clicontext.Bool("dry-run"),
	}

	replacer := makeReplacer(
		context.Background(),
		region,
		profile,
	)

	unusedsnapshots, err := replacer.SearchUnusedSnapshot(conf.Owner)
	sort.Sort(apis.VolumeSlice(unusedsnapshots.Snapshots))
	length := apis.VolumeSlice(unusedsnapshots.Snapshots).Len()
	for i := 0; i < length; i++ {
		id := *unusedsnapshots.Snapshots[i].SnapshotId
		snaps, err := replacer.ImageExists(id)
		if err != nil {
			return err
		}
		if snaps == nil {
			volumes, err := replacer.VolumeExists(id)
			if err != nil {
				return err
			}
			if volumes == nil {
				fmt.Println(id)
				deleteresult, err := replacer.DeleteSnapshot(id, dryrun)
				if err != nil {
					return err
				}
				fmt.Println(deleteresult)
			}
		}
	}
	return err
}

func replaceInstances(clicontext *cli.Context) error {
	conf := &config.Config{
		Asgname:     clicontext.String("asgname"),
		Clustername: clicontext.String("clustername"),
		Image:       clicontext.String("image"),
		Owner:       clicontext.String("owner"),
		Dryrun:      clicontext.Bool("dry-run"),
		Debug:       clicontext.Bool("verbose"),
	}

	if conf.Debug {
		log.SetLevel("debug")
	}

	_, err := config.ParseRegion(region)
	if err != nil {
		return fmt.Errorf("aws region is invalid! %v", err)
	}

	if !config.IsValidProfile(profile) {
		return fmt.Errorf("Invalid Config")
	}

	replacer := makeReplacer(
		context.Background(),
		region,
		profile,
	)

	instances, err := replacer.ReplaceInstance(conf.Asgname, conf.Clustername, conf.Dryrun, conf.Image, conf.Owner)
	if err != nil {
		return fmt.Errorf("failed to replace instance. %v", err)
	}
	log.Debug.Println(instances)
	return nil
}
