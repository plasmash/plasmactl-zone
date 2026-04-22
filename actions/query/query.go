package query

import (
	"fmt"
	"sort"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-zone/pkg/topology"
	"github.com/plasmash/plasmactl-component/pkg/component"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// QueryResult is the structured output for zone:query
type QueryResult struct {
	Paths []string `json:"paths"`
}

// Query implements the zone:query command
type Query struct {
	action.WithLogger
	action.WithTerm

	Dir        string
	Identifier string
	Kind       string // "node" or "component" to narrow search

	result *QueryResult
}

// Execute runs the query action
func (q *Query) Execute() error {
	// Load topology for distribution computation
	t, err := topology.Load(q.Dir)
	if err != nil {
		return err
	}

	var zonePaths []string

	// Search based on kind or search both when unspecified
	searchNode := q.Kind == "" || q.Kind == "node"
	searchComponent := q.Kind == "" || q.Kind == "component"

	if q.Kind != "" && !searchNode && !searchComponent {
		return fmt.Errorf("invalid kind %q: must be \"node\" or \"component\"", q.Kind)
	}

	// Search in nodes (allocations with distribution)
	if searchNode {
		nodesByPlatform, err := node.LoadByPlatform(q.Dir)
		if err != nil {
			q.Log().Debug("Failed to load nodes", "error", err)
		}

		for _, nodes := range nodesByPlatform {
			// Compute effective allocations for all nodes in this platform
			allocations := nodes.Allocations(t)

			for _, n := range nodes {
				if n.Hostname == q.Identifier {
					// Use effective allocations (after distribution)
					zonePaths = append(zonePaths, allocations[n.Hostname]...)
				}
			}
		}
	}

	// Search in attachments (components) -- always search when applicable, no short-circuit
	if searchComponent {
		components, err := component.LoadFromPlaybooks(q.Dir)
		if err != nil {
			q.Log().Debug("Failed to load components", "error", err)
		}

		attachmentsMap := components.Attachments(t)
		if attached, ok := attachmentsMap[q.Identifier]; ok {
			zonePaths = append(zonePaths, attached...)
		}
	}

	if len(zonePaths) == 0 {
		return fmt.Errorf("no zone paths found for %q (searched as %s)", q.Identifier, q.searchDescription())
	}

	// Remove duplicates and sort
	seen := make(map[string]bool)
	var unique []string
	for _, p := range zonePaths {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	sort.Strings(unique)

	q.result = &QueryResult{Paths: unique}

	for _, s := range unique {
		q.Term().Printfln("%s", s)
	}

	return nil
}

// searchDescription returns a human-readable description of what was searched.
func (q *Query) searchDescription() string {
	switch q.Kind {
	case "node":
		return "node"
	case "component":
		return "component"
	default:
		return "node and component"
	}
}

// Result returns the structured result for JSON output
func (q *Query) Result() any {
	return q.result
}
