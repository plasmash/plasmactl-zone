package show

import (
	"fmt"
	"sort"
	"strings"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-zone/pkg/topology"
	"github.com/plasmash/plasmactl-component/pkg/component"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// AllocationInfo represents a node allocation
type AllocationInfo struct {
	Node     string   `json:"node"`
	Platform string   `json:"platform"`
	Zones    []string `json:"zones"`
}

// DisplayName returns the node formatted as "hostname@platform".
func (a AllocationInfo) DisplayName() string {
	return a.Node + "@" + a.Platform
}

// AttachmentInfo represents a component attachment
type AttachmentInfo struct {
	Component string `json:"component"`
	Version   string `json:"version,omitempty"`
	Zone      string `json:"zone"`
}

// DisplayName returns the component formatted as "name@version".
func (a AttachmentInfo) DisplayName() string {
	return component.FormatDisplayName(a.Component, a.Version)
}

// ShowResult is the structured output for zone:show
type ShowResult struct {
	Zone        string           `json:"zone,omitempty"`
	Allocations []AllocationInfo `json:"allocations,omitempty"`
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
}

// Show implements the zone:show command
type Show struct {
	action.WithLogger
	action.WithTerm

	Dir      string
	Zone     string
	Platform string
	Kind     string // "allocations" or "attachments" to filter

	result *ShowResult
}

// Result returns the structured result for JSON output
func (s *Show) Result() any {
	return s.result
}

// Execute runs the show action
func (s *Show) Execute() error {
	t, err := topology.Load(s.Dir)
	if err != nil {
		return err
	}

	// If zone path specified, validate it exists
	if s.Zone != "" && !t.Exists(s.Zone) {
		return fmt.Errorf("zone %q not found in topology.yaml", s.Zone)
	}

	showAllocations := s.Kind == "" || s.Kind == "allocations"
	showAttachments := s.Kind == "" || s.Kind == "attachments"

	// Load all nodes by platform
	nodesByPlatform, err := node.LoadByPlatform(s.Dir)
	if err != nil {
		s.Log().Debug("Failed to load nodes", "error", err)
	}

	// Filter by platform if specified
	if s.Platform != "" {
		filtered := make(map[string]node.Nodes)
		if nodes, ok := nodesByPlatform[s.Platform]; ok {
			filtered[s.Platform] = nodes
		}
		nodesByPlatform = filtered
	}

	// Load components from playbooks
	components, err := component.LoadFromPlaybooks(s.Dir)
	if err != nil {
		s.Log().Debug("Failed to load components", "error", err)
	}

	// Build version map for quick lookup
	versionMap := make(map[string]string)
	for _, comp := range components {
		versionMap[comp.Name] = comp.Version
	}

	// Get attachments map (component -> zone paths)
	attachmentsMap := components.Attachments(t)

	// Collect component attachments for the zone path
	type componentInfo struct {
		zone      string
		component string
		version   string
	}
	var compInfos []componentInfo

	for compName, zonePaths := range attachmentsMap {
		for _, zonePath := range zonePaths {
			// Check if zone path matches query (exact match or descendant)
			if s.Zone == "" || zonePath == s.Zone || topology.IsDescendantOf(zonePath, s.Zone) {
				compInfos = append(compInfos, componentInfo{
					zone:      zonePath,
					component: compName,
					version:   versionMap[compName],
				})
			}
		}
	}

	// Sort components by zone path, then component name
	sort.Slice(compInfos, func(i, j int) bool {
		if compInfos[i].zone != compInfos[j].zone {
			return compInfos[i].zone < compInfos[j].zone
		}
		return compInfos[i].component < compInfos[j].component
	})

	// Collect all node allocations (EFFECTIVE - after distribution)
	type nodeInfo struct {
		platform string
		node     string
		zones    []string // effective zone paths after distribution
	}
	var nodes []nodeInfo

	// Get sorted platform names
	var platforms []string
	for platform := range nodesByPlatform {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)

	for _, platform := range platforms {
		platformNodes := nodesByPlatform[platform]

		// Compute effective allocations for all nodes in this platform
		allocations := platformNodes.Allocations(t)

		for _, n := range platformNodes {
			effectiveZones := allocations[n.Hostname]

			// If zone filter is specified, check if node is allocated to it
			if s.Zone != "" {
				found := false
				for _, zonePath := range effectiveZones {
					if zonePath == s.Zone || topology.IsDescendantOf(zonePath, s.Zone) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			nodes = append(nodes, nodeInfo{
				platform: platform,
				node:     n.Hostname,
				zones:    effectiveZones,
			})
		}
	}

	// Sort nodes by platform, then node
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].platform != nodes[j].platform {
			return nodes[i].platform < nodes[j].platform
		}
		return nodes[i].node < nodes[j].node
	})

	// Build result
	s.result = &ShowResult{
		Zone: s.Zone,
	}

	for _, n := range nodes {
		s.result.Allocations = append(s.result.Allocations, AllocationInfo{
			Node:     n.node,
			Platform: n.platform,
			Zones:    n.zones,
		})
	}

	for _, comp := range compInfos {
		s.result.Attachments = append(s.result.Attachments, AttachmentInfo{
			Component: comp.component,
			Version:   comp.version,
			Zone:      comp.zone,
		})
	}

	// Output
	hasAllocations := showAllocations && len(s.result.Allocations) > 0
	hasAttachments := showAttachments && len(s.result.Attachments) > 0

	if !hasAllocations && !hasAttachments {
		s.Term().Info().Println("No allocations or attachments found")
		return nil
	}

	if hasAllocations {
		s.Term().Info().Printfln("Allocations (%d nodes)", len(s.result.Allocations))
		for _, n := range s.result.Allocations {
			zonesStr := strings.Join(n.Zones, ", ")
			if len(zonesStr) > 60 {
				zonesStr = zonesStr[:57] + "..."
			}
			s.Term().Printfln("  %s  [%s]", n.DisplayName(), zonesStr)
		}
	}

	if hasAttachments {
		s.Term().Info().Printfln("Attachments (%d components)", len(s.result.Attachments))
		for _, a := range s.result.Attachments {
			s.Term().Printfln("  %s  @ %s", a.DisplayName(), a.Zone)
		}
	}

	return nil
}
