package list

import (
	"sort"
	"strings"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-zone/pkg/topology"
	"github.com/plasmash/plasmactl-component/pkg/component"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// TreeEntry enriches a zone path with its allocated nodes and attached components.
type TreeEntry struct {
	Path       string   `json:"path"`
	Nodes      []string `json:"nodes,omitempty"`
	Components []string `json:"components,omitempty"`
}

// ListResult is the structured output for zone:list
type ListResult struct {
	Zones []string    `json:"zones"`
	Tree  []TreeEntry `json:"tree,omitempty"`
}

// List implements the zone:list command
type List struct {
	action.WithLogger
	action.WithTerm

	Dir  string
	Zone string
	Tree bool

	result *ListResult
}

// Result returns the structured result for JSON output
func (l *List) Result() any {
	return l.result
}

// Execute runs the list action
func (l *List) Execute() error {
	t, err := topology.Load(l.Dir)
	if err != nil {
		return err
	}

	// Initialize result early so --json always returns an object, never null
	l.result = &ListResult{Zones: []string{}}

	paths := t.FlattenWithPrefix(l.Zone)
	if len(paths) == 0 {
		l.Term().Warning().Println("No zone paths found")
		return nil
	}

	l.result.Zones = paths

	if l.Tree {
		l.printTreeWithRelations(t, paths)
	} else {
		// Flat output - one per line, scriptable
		for _, z := range l.result.Zones {
			l.Term().Printfln("%s", z)
		}
	}

	return nil
}


// printTreeWithRelations prints the topology tree with nodes and components inline
func (l *List) printTreeWithRelations(t *topology.Topology, paths []string) {
	// Load nodes and compute allocations
	nodesByPlatform, err := node.LoadByPlatform(l.Dir)
	if err != nil {
		l.Log().Debug("Failed to load nodes", "error", err)
	}
	zoneToNodes := make(map[string][]string)

	for _, nodes := range nodesByPlatform {
		allocations := nodes.Allocations(t)
		for _, n := range nodes {
			for _, zonePath := range allocations[n.Hostname] {
				zoneToNodes[zonePath] = append(zoneToNodes[zonePath], n.DisplayName())
			}
		}
	}

	// Load components
	components, err := component.LoadFromPlaybooks(l.Dir)
	if err != nil {
		l.Log().Debug("Failed to load components", "error", err)
	}
	zoneToComponents := make(map[string][]string)
	for _, comp := range components {
		zoneToComponents[comp.Zone] = append(zoneToComponents[comp.Zone], comp.Name)
	}

	// Sort the relations for consistent output
	for zonePath := range zoneToNodes {
		sort.Strings(zoneToNodes[zonePath])
	}
	for zonePath := range zoneToComponents {
		sort.Strings(zoneToComponents[zonePath])
	}

	// Populate tree entries in result
	for _, p := range paths {
		entry := TreeEntry{Path: p}
		if nodes, ok := zoneToNodes[p]; ok {
			entry.Nodes = nodes
		}
		if comps, ok := zoneToComponents[p]; ok {
			entry.Components = comps
		}
		l.result.Tree = append(l.result.Tree, entry)
	}

	// Build tree structure
	tree := buildTree(paths)

	// Print tree starting from root's children
	for _, child := range tree.children {
		printNodeWithRelations(l.Term(), child, "", "", zoneToNodes, zoneToComponents)
	}
}

type treeNode struct {
	name     string
	fullPath string
	children []*treeNode
}

func buildTree(paths []string) *treeNode {
	root := &treeNode{name: ""}

	for _, path := range paths {
		parts := strings.Split(path, ".")
		current := root
		currentPath := ""
		for _, part := range parts {
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = currentPath + "." + part
			}

			found := false
			for _, child := range current.children {
				if child.name == part {
					current = child
					found = true
					break
				}
			}
			if !found {
				newNode := &treeNode{name: part, fullPath: currentPath}
				current.children = append(current.children, newNode)
				current = newNode
			}
		}
	}

	return root
}

func printNodeWithRelations(term *launchr.Terminal, node *treeNode, indent, prefix string, zoneToNodes, zoneToComponents map[string][]string) {
	// Print this node
	term.Printfln("%s%s", prefix, node.name)

	// Get nodes and components for this zone path
	nodes := zoneToNodes[node.fullPath]
	comps := zoneToComponents[node.fullPath]

	// Order: child zone paths first (structural hierarchy), then nodes, then components
	totalChildren := len(node.children) + len(nodes) + len(comps)
	childIdx := 0

	// Print child zone paths first
	for _, child := range node.children {
		childIdx++
		isLast := childIdx == totalChildren

		var childPrefix, nextIndent string
		if isLast {
			childPrefix = indent + "└── "
			nextIndent = indent + "    "
		} else {
			childPrefix = indent + "├── "
			nextIndent = indent + "│   "
		}

		printNodeWithRelations(term, child, nextIndent, childPrefix, zoneToNodes, zoneToComponents)
	}

	// Print nodes allocated to this zone path
	for _, n := range nodes {
		childIdx++
		isLast := childIdx == totalChildren
		var childPrefix string
		if isLast {
			childPrefix = indent + "└── "
		} else {
			childPrefix = indent + "├── "
		}
		term.Printfln("%s🖥 %s", childPrefix, n)
	}

	// Print components distributed to this zone path
	for _, comp := range comps {
		childIdx++
		isLast := childIdx == totalChildren
		var childPrefix string
		if isLast {
			childPrefix = indent + "└── "
		} else {
			childPrefix = indent + "├── "
		}
		term.Printfln("%s🧩 %s", childPrefix, comp)
	}
}
