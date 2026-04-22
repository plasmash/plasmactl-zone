package remove

import (
	"fmt"
	"strings"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-zone/internal/topology"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// RemoveResult is the structured result of zone:remove.
type RemoveResult struct {
	Zone               string   `json:"zone"`
	DryRun             bool     `json:"dry_run,omitempty"`
	AllocatedNodes     []string `json:"allocated_nodes,omitempty"`
	AttachedComponents []string `json:"attached_components,omitempty"`
}

// Remove implements the zone:remove command
type Remove struct {
	action.WithLogger
	action.WithTerm

	Dir    string
	Zone   string
	DryRun bool

	result *RemoveResult
}

// Result returns the structured result for JSON output.
func (r *Remove) Result() any {
	return r.result
}

// Execute runs the remove action
func (r *Remove) Execute() error {
	t, err := topology.Load(r.Dir)
	if err != nil {
		return err
	}

	if !t.Exists(r.Zone) {
		return fmt.Errorf("zone %q not found", r.Zone)
	}

	// Check for allocated nodes using distributed allocations
	nodesByPlatform, err := node.LoadByPlatform(r.Dir)
	if err != nil {
		r.Log().Debug("Failed to load nodes", "error", err)
	}

	var allocatedNodes []string
	for _, nodes := range nodesByPlatform {
		allocations := nodes.Allocations(t.Topology)
		for _, n := range nodes {
			for _, zp := range allocations[n.Hostname] {
				if zp == r.Zone || strings.HasPrefix(zp, r.Zone+".") {
					allocatedNodes = append(allocatedNodes, n.DisplayName())
					break
				}
			}
		}
	}

	// Check for attached components
	attachments, err := topology.LoadAttachments(r.Dir, r.Zone)
	if err != nil {
		r.Log().Debug("Failed to load attachments", "error", err)
	}

	var attachedComponents []string
	for _, a := range attachments {
		attachedComponents = append(attachedComponents, a.Component)
	}

	// Dry-run: report what would block removal
	if r.DryRun {
		r.result = &RemoveResult{
			Zone:               r.Zone,
			DryRun:             true,
			AllocatedNodes:     allocatedNodes,
			AttachedComponents: attachedComponents,
		}

		r.Term().Info().Println("[dry-run] No changes will be made")
		if len(allocatedNodes) > 0 {
			r.Term().Info().Println("Allocated nodes:")
			for _, n := range allocatedNodes {
				r.Term().Printfln("  %s", n)
			}
		}
		if len(attachedComponents) > 0 {
			r.Term().Info().Println("Attached components:")
			for _, comp := range attachedComponents {
				r.Term().Printfln("  %s", comp)
			}
		}
		if len(allocatedNodes) == 0 && len(attachedComponents) == 0 {
			r.Term().Success().Printfln("Safe to remove: %s", r.Zone)
		}
		return nil
	}

	// Check blockers
	if len(allocatedNodes) > 0 {
		r.Term().Info().Println("Allocated nodes:")
		for _, n := range allocatedNodes {
			r.Term().Printfln("  %s", n)
		}
		return fmt.Errorf("cannot remove zone %q: %d node(s) are allocated (deallocate them first)", r.Zone, len(allocatedNodes))
	}

	if len(attachedComponents) > 0 {
		r.Term().Info().Println("Attached components:")
		for _, comp := range attachedComponents {
			r.Term().Printfln("  %s", comp)
		}
		return fmt.Errorf("cannot remove zone %q: %d component(s) are attached (detach them first)", r.Zone, len(attachedComponents))
	}

	// Safe to remove
	if err := t.Remove(r.Zone); err != nil {
		return err
	}

	if err := t.Save(r.Dir); err != nil {
		return err
	}

	r.result = &RemoveResult{Zone: r.Zone}
	r.Term().Success().Printfln("Removed: %s", r.Zone)
	return nil
}
