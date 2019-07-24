package actions

import (
	"fmt"
	"sort"

	"github.com/nest-egg/ami-replacer/apis"
	"github.com/nest-egg/ami-replacer/config"
	"github.com/nest-egg/ami-replacer/log"
	"golang.org/x/xerrors"
)

var (
	state  string
	dryrun bool
)

//ReplaceInstance replace ecs cluster instances with newest amis.
func (r *Replacement) ReplaceInstance(c *config.Config) error {

	dryrun = c.Dryrun

	clst, err := r.setClusterStatus(c)
	if err != nil {
		return xerrors.Errorf("failed to set cluster status: %w", err)
	}

	defaultClusterSize := clst.size

	if len(clst.unusedInstances) != 0 {
		if err := r.deploy.FSM.Event("start"); err != nil {
			return xerrors.New("failed to enter state")
		}
		_, err := r.replaceUnusedInstance(clst)
		if err != nil {
			return xerrors.Errorf("failed to replace unused instance: %w", err)
		}
		if err := r.deploy.FSM.Event("finish"); err != nil {
			return xerrors.New("failed to enter state")
		}
		clst, err = r.refreshClusterStatus(clst)
		if err != nil {
			return xerrors.Errorf("failed to refresh cluster status: %w", err)
		}
	}

	state = r.deploy.FSM.Current()

	if len(clst.freeInstances) == 0 && state == "closed" {
		log.Info.Printf("cluster %v has no empty ECS instances", clst.name)
		log.Info.Printf("extend the size of the cluster.. current size: %d", clst.size)
		if clst.size+1 > defaultClusterSize {
			if err := r.optimizeClusterSize(clst, clst.size+1); err != nil {
				return xerrors.Errorf("failed to increase asg size: %w", err)
			}
		} else if clst.size+1 <= defaultClusterSize {
			if err := r.waitInstanceRunning(clst, defaultClusterSize); err != nil {
				return xerrors.Errorf("failed to execute waiter: %w", err)
			}
		}

		clst, err = r.refreshClusterStatus(clst)
		if err != nil {
			return xerrors.Errorf("failed to refresh cluster status: %w", err)
		}
	}

	if len(clst.ecsInstance) != 0 && state == "closed" {
		if err := r.swapInstance(clst); err != nil {
			return xerrors.Errorf("failed to swap cluster instance: %w", err)
		}
	}

	state = r.deploy.FSM.Current()
	if state != "closed" {
		return fmt.Errorf("cluster is not steady state")
	} else if state == "closed" {
		if err := r.optimizeClusterSize(clst, defaultClusterSize); err != nil {
			return xerrors.Errorf("failed to decrease asg size: %w", err)
		}
		log.Info.Println("successfully restored the size of the cluster")

	}
	return nil
}

//RemoveSnapShots removes obsolete snapshots.
func (r *Replacement) RemoveSnapShots(c *config.Config) error {

	dryrun = c.Dryrun
	result, err := r.searchUnusedSnapshot(c.Owner)
	if err != nil {
		return xerrors.Errorf("failed to searcj unused instance: %w", err)
	}
	sort.Sort(apis.VolumeSlice(result.Snapshots))
	length := apis.VolumeSlice(result.Snapshots).Len()
	log.Info.Printf("%d snapshots found", length)
	for i := 0; i < length; i++ {
		id := *result.Snapshots[i].SnapshotId
		snaps, err := r.imageExists(id)
		if err != nil {
			return xerrors.Errorf("failed to get existing image: %w", err)
		}
		if snaps.Images == nil {
			volumes, err := r.volumeExists(id)
			if err != nil {
				return xerrors.Errorf("failed to get exiting volume: %w", err)
			}
			if volumes.Volumes == nil {
				log.Info.Printf("Delete snapshot: %v", id)
				_, err := r.deleteSnapshot(id)
				if err != nil {
					return xerrors.Errorf("failed to delete snapshot: %w", err)
				}
			}
		}
	}
	return nil
}

//RemoveAMIs removes obsolete AMIs
func (r *Replacement) RemoveAMIs(c *config.Config) error {

	dryrun = c.Dryrun
	output, err := r.deregisterAMI(c)
	_ = output
	if err != nil {
		return fmt.Errorf("deregister failed! %v", err)
	}
	return nil
}
