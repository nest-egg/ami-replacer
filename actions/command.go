package actions

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/nest-egg/ami-replacer/log"
	"github.com/nest-egg/ami-replacer/config"

	"github.com/nest-egg/ami-replacer/apis"
)

//state of asg
var state string

//ReplaceInstance replace ecs cluster instances with newest amis.
func (r *Replacement) ReplaceInstance(c *config.Config) (grp *autoscaling.Group, err error) {

	newestimage, err := r.NewestAMI(c.Owner, c.Image)
	if err != nil {
		return nil, err
	}
	clst, err := r.setClusterStatus(c.Asgname, c.Clustername, newestimage)
	if err != nil {
		return nil, err
	}
	defaultClusterSize := clst.ClusterSize

	if len(clst.UnusedInstances) != 0 {
		if err := r.deploy.FSM.Event("start"); err != nil {
			return nil, err
		}
		_, err := r.replaceUnusedInstance(c.Asgname, clst.UnusedInstances, c.Dryrun)
		if err != nil {
			return nil, fmt.Errorf("cannnot stop unused instance: %v", err)
		}
		if err := r.deploy.FSM.Event("finish"); err != nil {
			return nil, err
		}
		clst, err = r.setClusterStatus(c.Asgname, c.Clustername, newestimage)
		if err != nil {
			return nil, err
		}
	}

	state = r.deploy.FSM.Current()

	if len(clst.FreeInstances) == 0 && state == "closed" {
		log.Info.Println("cluster has no empty ECS instances")
		log.Info.Printf("extend the size of the cluster.. current size: %d", clst.ClusterSize)
		if err := r.optimizeClusterSize(clst, c.Asgname, clst.ClusterSize+1); err != nil {
			return nil, err
		}
		clst, err = r.setClusterStatus(c.Asgname, c.Clustername, newestimage)
		if err != nil {
			return nil, err
		}
	}

	if len(clst.EcsInstance) != 0 && state == "closed" {
		_, err := r.swapInstance(clst, newestimage, c.Dryrun)
		if err != nil {
			return nil, err
		}
	}

	state = r.deploy.FSM.Current()
	if state != "closed" {
		return nil, fmt.Errorf("cluster is not steady state")
	} else if state == "closed" {
		if err := r.optimizeClusterSize(clst, c.Asgname, defaultClusterSize); err != nil {
			return nil, err
		}
		log.Info.Println("successfully recovered the size of the cluster")

	}
	return nil, nil
}

//RemoveSnapShots removes obsolete snapshots.
func(r *Replacement)RemoveSnapShots(c *config.Config)error{

	unusedsnapshots, err := r.SearchUnusedSnapshot(c.Owner)
	sort.Sort(apis.VolumeSlice(unusedsnapshots.Snapshots))
	length := apis.VolumeSlice(unusedsnapshots.Snapshots).Len()
	for i := 0; i < length; i++ {
		id := *unusedsnapshots.Snapshots[i].SnapshotId
		snaps, err := r.ImageExists(id)
		if err != nil {
			return err
		}
		if snaps == nil {
			volumes, err := r.VolumeExists(id)
			if err != nil {
				return err
			}
			if volumes == nil {
				fmt.Println(id)
				_, err := r.DeleteSnapshot(id, c.Dryrun)
				if err != nil {
					return err
				}
			}
		}
	}
	return err
}

//RemoveAMIs removes obsolete AMIs
func(r *Replacement)RemoveAMIs(c *config.Config)error{

	asginstance, err := r.asgInfo(c.Asgname)
	instanceid := asginstance.Instances[0].InstanceId
	imageid, err := r.Ami(*instanceid)
	log.Debug.Println(imageid)

	output, err := r.DeregisterAMI(imageid, c.Owner, c.Image, c.Generation, c.Dryrun)
	_=output
	if err != nil {
		log.Fatalf("deregister failed! %v", err)
	}
	return nil
}