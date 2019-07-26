package fsm

import (
	"github.com/looplab/fsm"
	"github.com/nest-egg/ami-replacer/log"
)

// Deploy defines ASG state while deploying.
type Deploy struct {
	To  string
	FSM *fsm.FSM
}

// State is current cluster state information
type State struct {
	Success bool   `json:"success"`
	Current string `json:"current"`
	Last    string `json:"last"`
	RUNNUM  int    `json:"runnum"`
	STOPNUM int    `json:"stopnum"`
}

//NewDeploy create FSM for ASG state.
func NewDeploy(to string) *Deploy {

	d := &Deploy{
		To: to,
	}

	d.FSM = fsm.NewFSM(
		"closed",
		fsm.Events{
			{Name: "start", Src: []string{"closed"}, Dst: "running"},
			{Name: "finish", Src: []string{"running"}, Dst: "closed"},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { d.enterState(e) },
		},
	)
	return d
}

func (d *Deploy) enterState(e *fsm.Event) {
	log.Logger.Infof("the state changed %s to %s\n", d.To, e.Dst)
}
