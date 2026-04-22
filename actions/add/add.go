package add

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-zone/internal/topology"
)

// AddResult is the structured result of zone:add.
type AddResult struct {
	Zone string `json:"zone"`
}

// Add implements the zone:add command
type Add struct {
	action.WithLogger
	action.WithTerm

	Dir   string
	Zone  string
	Force bool

	result *AddResult
}

// Result returns the structured result for JSON output.
func (a *Add) Result() any {
	return a.result
}

// Execute runs the add action
func (a *Add) Execute() error {
	t, err := topology.Load(a.Dir)
	if err != nil {
		return err
	}

	if a.Force && t.Exists(a.Zone) {
		a.result = &AddResult{Zone: a.Zone}
		a.Term().Info().Printfln("Already exists: %s", a.Zone)
		return nil
	}

	if err := t.Add(a.Zone); err != nil {
		return fmt.Errorf("failed to add zone path: %w", err)
	}

	if err := t.Save(a.Dir); err != nil {
		return err
	}

	a.result = &AddResult{Zone: a.Zone}
	a.Term().Success().Printfln("Added: %s", a.Zone)
	return nil
}
