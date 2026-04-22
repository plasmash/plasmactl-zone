package rename

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-zone/internal/topology"
)

// RenameResult is the structured result of zone:rename.
type RenameResult struct {
	Old                string   `json:"old"`
	New                string   `json:"new"`
	DryRun             bool     `json:"dry_run,omitempty"`
	UpdatedAttachments []string `json:"updated_attachments,omitempty"`
	UpdatedAllocations []string `json:"updated_allocations,omitempty"`
}

// Rename implements the zone:rename command
type Rename struct {
	action.WithLogger
	action.WithTerm

	Dir    string
	Old    string
	New    string
	DryRun bool

	result *RenameResult
}

// Result returns the structured result for JSON output.
func (r *Rename) Result() any {
	return r.result
}

// Execute runs the rename action
func (r *Rename) Execute() error {
	t, err := topology.Load(r.Dir)
	if err != nil {
		return err
	}

	if !t.Exists(r.Old) {
		return fmt.Errorf("zone %q does not exist", r.Old)
	}

	if t.Exists(r.New) {
		return fmt.Errorf("zone %q already exists", r.New)
	}

	if r.DryRun {
		return r.executeDryRun()
	}

	// Rename in topology.yaml
	if err := t.Rename(r.Old, r.New); err != nil {
		return fmt.Errorf("failed to rename zone path: %w", err)
	}

	if err := t.Save(r.Dir); err != nil {
		return err
	}

	// Update attachments
	updatedAttachments, err := topology.UpdateAttachments(r.Dir, r.Old, r.New)
	if err != nil {
		r.Term().Warning().Printfln("Zone renamed but failed to update attachments: %s", err)
	}

	// Update allocations
	updatedAllocations, err := topology.UpdateAllocations(r.Dir, r.Old, r.New)
	if err != nil {
		r.Term().Warning().Printfln("Zone renamed but failed to update allocations: %s", err)
	}

	r.result = &RenameResult{
		Old:                r.Old,
		New:                r.New,
		UpdatedAttachments: updatedAttachments,
		UpdatedAllocations: updatedAllocations,
	}

	r.Term().Success().Printfln("Renamed: %s -> %s", r.Old, r.New)
	if len(updatedAttachments) > 0 {
		r.Term().Info().Println("Updated attachments:")
		for _, p := range updatedAttachments {
			r.Term().Printfln("  - %s", p)
		}
	}
	if len(updatedAllocations) > 0 {
		r.Term().Info().Println("Updated allocations:")
		for _, p := range updatedAllocations {
			r.Term().Printfln("  - %s", p)
		}
	}

	return nil
}

// executeDryRun shows what would change without modifying any files.
func (r *Rename) executeDryRun() error {
	r.Term().Info().Println("[dry-run] No changes will be made")
	r.Term().Printfln("  topology.yaml: %s -> %s", r.Old, r.New)

	// Find affected attachment files
	attachments, err := topology.LoadAttachments(r.Dir, r.Old)
	if err != nil {
		r.Log().Debug("Failed to load attachments", "error", err)
	}

	seen := make(map[string]bool)
	var affectedPlaybooks []string
	for _, a := range attachments {
		if !seen[a.Playbook] {
			seen[a.Playbook] = true
			affectedPlaybooks = append(affectedPlaybooks, a.Playbook)
		}
	}

	// Find affected allocation files
	nodesByPlatform, err := topology.LoadNodesByPlatform(r.Dir)
	if err != nil {
		r.Log().Debug("Failed to load nodes", "error", err)
	}

	var affectedNodeFiles []string
	for platform, nodes := range nodesByPlatform {
		for _, n := range topology.NodesForZone(nodes, r.Old) {
			affectedNodeFiles = append(affectedNodeFiles, fmt.Sprintf("platforms/%s/nodes/%s.yaml", platform, n.Hostname))
		}
	}

	if len(affectedPlaybooks) > 0 {
		r.Term().Info().Println("Would update attachments:")
		for _, p := range affectedPlaybooks {
			r.Term().Printfln("  - %s", p)
		}
	}
	if len(affectedNodeFiles) > 0 {
		r.Term().Info().Println("Would update allocations:")
		for _, p := range affectedNodeFiles {
			r.Term().Printfln("  - %s", p)
		}
	}

	r.result = &RenameResult{
		Old:                r.Old,
		New:                r.New,
		DryRun:             true,
		UpdatedAttachments: affectedPlaybooks,
		UpdatedAllocations: affectedNodeFiles,
	}

	return nil
}
