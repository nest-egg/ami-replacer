package actions

import (
	"fmt"
	"sort"

	"github.com/nest-egg/ami-replacer/apis"
	"github.com/nest-egg/ami-replacer/config"
	"github.com/nest-egg/ami-replacer/log"
)

//state of asg
var (
	state  string
	dryrun bool
)

//ReplaceInstance replace ecs cluster instances with newest amis.
func (r *Replacement) ReplaceInstance(c *config.Config) error {

	dryrun = c.Dryrun

	clst, err := r.setClusterStatus(c)
	if err != nil {
		return err
	}

	defaultClusterSize := clst.size

	if len(clst.unusedInstances) != 0 {
		if err := r.deploy.FSM.Event("start"); err != nil {
			return err
		}
		_, err := r.replaceUnusedInstance(clst)
		if err != nil {
			return err
		}
		if err := r.deploy.FSM.Event("finish"); err != nil {
			return err
		}
		clst, err = r.refreshClusterStatus(clst)
		if err != nil {
			return err
		}
	}

	state = r.deploy.FSM.Current()

	if len(clst.freeInstances) == 0 && state == "closed" {
		log.Info.Printf("cluster %v has no empty ECS instances", clst.name)
		log.Info.Printf("extend the size of the cluster.. current size: %d", clst.size)
		if err := r.optimizeClusterSize(clst, clst.size+1); err != nil {
			return err
		}
		clst, err = r.refreshClusterStatus(clst)
		if err != nil {
			return err
		}
	}

	if len(clst.ecsInstance) != 0 && state == "closed" {
		if err := r.swapInstance(clst); err != nil {
			return err
		}
	}

	state = r.deploy.FSM.Current()
	if state != "closed" {
		return fmt.Errorf("cluster is not steady state")
	} else if state == "closed" {
		if err := r.optimizeClusterSize(clst, defaultClusterSize); err != nil {
			return err
		}
		log.Info.Println("successfully recovered the size of the cluster")

	}
	return nil
}

//RemoveSnapShots removes obsolete snapshots.
func (r *Replacement) RemoveSnapShots(c *config.Config) error {

	dryrun = c.Dryrun
	unusedsnapshots, err := r.searchUnusedSnapshot(c.Owner)
	sort.Sort(apis.VolumeSlice(unusedsnapshots.Snapshots))
	length := apis.VolumeSlice(unusedsnapshots.Snapshots).Len()
	for i := 0; i < length; i++ {
		id := *unusedsnapshots.Snapshots[i].SnapshotId
		snaps, err := r.imageExists(id)
		if err != nil {
			return err
		}
		if snaps == nil {
			volumes, err := r.volumeExists(id)
			if err != nil {
				return err
			}
			if volumes == nil {
				fmt.Println(id)
				_, err := r.deleteSnapshot(id)
				if err != nil {
					return err
				}
			}
		}
	}
	return err
}

//RemoveAMIs removes obsolete AMIs
func (r *Replacement) RemoveAMIs(c *config.Config) error {

	dryrun = c.Dryrun
	asginstance, err := r.asgInfo(c.Asgname)
	instanceid := asginstance.Instances[0].InstanceId
	imageid, err := r.Ami(*instanceid)
	log.Debug.Println(imageid)

	output, err := r.deregisterAMI(imageid, c.Owner, c.Image, c.Generation)
	_ = output
	if err != nil {
		log.Fatalf("deregister failed! %v", err)
	}
	return nil
}
