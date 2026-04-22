// Package topology provides shared logic for topology operations
package topology

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	pkgtopology "github.com/plasmash/plasmactl-zone/pkg/topology"
)

// Topology wraps the public Topology type with write operations.
type Topology struct {
	*pkgtopology.Topology
}

// Node represents a node file from platforms/<platform>/nodes/<hostname>.yaml
type Node struct {
	Hostname string   `yaml:"hostname"`
	Zones    []string `yaml:"zones"`
}

// Load reads and parses topology.yaml from the given directory
func Load(dir string) (*Topology, error) {
	pub, err := pkgtopology.Load(dir)
	if err != nil {
		return nil, err
	}
	return &Topology{Topology: pub}, nil
}

// Save writes the topology configuration to topology.yaml preserving order
func (t *Topology) Save(dir string) error {
	path := filepath.Join(dir, "topology.yaml")
	data, err := yaml.Marshal(t.YAMLNode())
	if err != nil {
		return fmt.Errorf("failed to marshal topology: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Add adds a new zone path preserving YAML order
// Path format: any dotted path (e.g., platform, platform.bite, platform.foundation.cluster)
func (t *Topology) Add(zonePath string) error {
	if err := pkgtopology.ValidatePath(zonePath); err != nil {
		return err
	}

	parts := strings.Split(zonePath, ".")

	if t.Exists(zonePath) {
		return fmt.Errorf("zone path %q already exists", zonePath)
	}

	// Work with yaml.Node to preserve order
	node := t.YAMLNode()
	if node == nil || len(node.Content) == 0 {
		// Create new document node
		newNode := &yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{{
				Kind: yaml.MappingNode,
			}},
		}
		t.SetYAMLNode(newNode)
		node = newNode
	}

	rootNode := node.Content[0]

	if len(parts) == 1 {
		// Just a root key (e.g., "platform")
		findOrCreateMapKey(rootNode, parts[0])
	} else if len(parts) == 2 {
		// Root and layer (e.g., "platform.bite")
		root := parts[0]
		layer := parts[1]
		rootValueNode := findOrCreateMapKey(rootNode, root)
		layerValueNode := findOrCreateMapKey(rootValueNode, layer)
		// Ensure it's a sequence node (empty)
		if layerValueNode.Kind != yaml.SequenceNode {
			layerValueNode.Kind = yaml.SequenceNode
			layerValueNode.Content = nil
		}
	} else {
		// Full path (e.g., "platform.foundation.cluster")
		root := parts[0]
		layer := parts[1]
		remaining := parts[2:]

		rootValueNode := findOrCreateMapKey(rootNode, root)
		layerValueNode := findOrCreateMapKey(rootValueNode, layer)

		// Ensure it's a sequence node
		if layerValueNode.Kind != yaml.SequenceNode {
			layerValueNode.Kind = yaml.SequenceNode
			layerValueNode.Content = nil
		}

		// Add the remaining path to the sequence
		addPathToSequence(layerValueNode, remaining)
	}

	// Also update data for consistency
	d := t.RawData()
	if d == nil {
		d = make(map[string]map[string][]interface{})
		t.SetRawData(d)
	}
	if len(parts) >= 2 {
		root := parts[0]
		layer := parts[1]
		if d[root] == nil {
			d[root] = make(map[string][]interface{})
		}
		if len(parts) > 2 {
			d[root][layer] = addZonePath(d[root][layer], parts[2:])
		} else {
			// Just ensure the layer exists
			if d[root][layer] == nil {
				d[root][layer] = []interface{}{}
			}
		}
	}

	return nil
}

// findOrCreateMapKey finds a key in a mapping node or creates it at the end
func findOrCreateMapKey(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode.Kind != yaml.MappingNode {
		mapNode.Kind = yaml.MappingNode
		mapNode.Content = nil
	}

	// Look for existing key
	for i := 0; i < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			return mapNode.Content[i+1]
		}
	}

	// Key not found, create at end
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	valueNode := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	mapNode.Content = append(mapNode.Content, keyNode, valueNode)
	return valueNode
}

// addPathToSequence adds a dotted path to a sequence node
func addPathToSequence(seqNode *yaml.Node, path []string) {
	if len(path) == 0 {
		return
	}

	name := path[0]
	remaining := path[1:]

	if len(remaining) == 0 {
		// Last segment - add as scalar
		// Check if it already exists
		for _, item := range seqNode.Content {
			if item.Kind == yaml.ScalarNode && item.Value == name {
				return // Already exists
			}
		}
		// Add new scalar at end
		seqNode.Content = append(seqNode.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: name,
		})
		return
	}

	// Need nested structure - look for existing map with this key
	for _, item := range seqNode.Content {
		if item.Kind == yaml.MappingNode {
			for i := 0; i < len(item.Content); i += 2 {
				if item.Content[i].Value == name {
					// Found existing key, recurse into its value
					valueNode := item.Content[i+1]
					if valueNode.Kind != yaml.SequenceNode {
						valueNode.Kind = yaml.SequenceNode
						valueNode.Content = nil
					}
					addPathToSequence(valueNode, remaining)
					return
				}
			}
		}
	}

	// Check if name exists as a scalar and convert it
	for i, item := range seqNode.Content {
		if item.Kind == yaml.ScalarNode && item.Value == name {
			// Convert scalar to map with sequence
			newSeq := &yaml.Node{Kind: yaml.SequenceNode}
			addPathToSequence(newSeq, remaining)
			seqNode.Content[i] = &yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
					newSeq,
				},
			}
			return
		}
	}

	// Create new map entry at end of sequence
	newSeq := &yaml.Node{Kind: yaml.SequenceNode}
	addPathToSequence(newSeq, remaining)
	seqNode.Content = append(seqNode.Content, &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			newSeq,
		},
	})
}

// Remove removes a zone path preserving YAML order
func (t *Topology) Remove(zonePath string) error {
	parts := strings.Split(zonePath, ".")
	if len(parts) < 1 || zonePath == "" {
		return fmt.Errorf("zone path cannot be empty")
	}

	if !t.Exists(zonePath) {
		return fmt.Errorf("zone path %q does not exist", zonePath)
	}

	// Remove from yaml.Node
	node := t.YAMLNode()
	if node != nil && len(node.Content) > 0 {
		rootNode := node.Content[0]
		if rootNode.Kind == yaml.MappingNode {
			if len(parts) == 1 {
				// Remove root key entirely
				root := parts[0]
				for i := 0; i < len(rootNode.Content); i += 2 {
					if rootNode.Content[i].Value == root {
						rootNode.Content = append(rootNode.Content[:i], rootNode.Content[i+2:]...)
						break
					}
				}
			} else if len(parts) == 2 {
				// Remove layer from root
				root := parts[0]
				layer := parts[1]
				for i := 0; i < len(rootNode.Content); i += 2 {
					if rootNode.Content[i].Value == root {
						rootValueNode := rootNode.Content[i+1]
						if rootValueNode.Kind == yaml.MappingNode {
							for j := 0; j < len(rootValueNode.Content); j += 2 {
								if rootValueNode.Content[j].Value == layer {
									rootValueNode.Content = append(rootValueNode.Content[:j], rootValueNode.Content[j+2:]...)
									break
								}
							}
						}
						break
					}
				}
			} else {
				// Remove from nested structure
				root := parts[0]
				layer := parts[1]
				remaining := parts[2:]
				for i := 0; i < len(rootNode.Content); i += 2 {
					if rootNode.Content[i].Value == root {
						rootValueNode := rootNode.Content[i+1]
						if rootValueNode.Kind == yaml.MappingNode {
							for j := 0; j < len(rootValueNode.Content); j += 2 {
								if rootValueNode.Content[j].Value == layer {
									layerValueNode := rootValueNode.Content[j+1]
									if layerValueNode.Kind == yaml.SequenceNode {
										removePathFromSequence(layerValueNode, remaining)
									}
									break
								}
							}
						}
						break
					}
				}
			}
		}
	}

	// Also update data for consistency
	d := t.RawData()
	if d == nil {
		return nil
	}

	if len(parts) == 1 {
		delete(d, parts[0])
		return nil
	}

	root := parts[0]
	layer := parts[1]

	if len(parts) == 2 {
		if d[root] != nil {
			delete(d[root], layer)
		}
		return nil
	}

	remaining := parts[2:]
	var removed bool
	d[root][layer], removed = removeZonePath(d[root][layer], remaining)
	if !removed {
		return fmt.Errorf("failed to remove zone path %q", zonePath)
	}

	return nil
}

// removePathFromSequence removes a dotted path from a sequence node
func removePathFromSequence(seqNode *yaml.Node, path []string) bool {
	if len(path) == 0 {
		return false
	}

	name := path[0]
	remaining := path[1:]

	for i, item := range seqNode.Content {
		if len(remaining) == 0 {
			// Looking for exact match to remove
			if item.Kind == yaml.ScalarNode && item.Value == name {
				seqNode.Content = append(seqNode.Content[:i], seqNode.Content[i+1:]...)
				return true
			}
			if item.Kind == yaml.MappingNode {
				for j := 0; j < len(item.Content); j += 2 {
					if item.Content[j].Value == name {
						// Remove entire map entry or just the key
						if len(item.Content) == 2 {
							// Only this key in map, remove the whole map item
							seqNode.Content = append(seqNode.Content[:i], seqNode.Content[i+1:]...)
						} else {
							// Multiple keys, just remove this one
							item.Content = append(item.Content[:j], item.Content[j+2:]...)
						}
						return true
					}
				}
			}
		} else {
			// Need to recurse
			if item.Kind == yaml.MappingNode {
				for j := 0; j < len(item.Content); j += 2 {
					if item.Content[j].Value == name {
						valueNode := item.Content[j+1]
						if valueNode.Kind == yaml.SequenceNode {
							return removePathFromSequence(valueNode, remaining)
						}
					}
				}
			}
		}
	}

	return false
}

// GetTree returns the topology as a tree structure for display
func (t *Topology) GetTree() map[string]interface{} {
	tree := make(map[string]interface{})
	d := t.RawData()
	for root, layers := range d {
		for layer, zones := range layers {
			tree[root+"."+layer] = zoneToTree(zones)
		}
	}
	return tree
}

// addZonePath adds a zone path to the nested structure
func addZonePath(zones []interface{}, path []string) []interface{} {
	if len(path) == 0 {
		return zones
	}

	name := path[0]
	remaining := path[1:]

	// If this is the last segment, add as string
	if len(remaining) == 0 {
		// Check if it already exists
		for _, z := range zones {
			if str, ok := z.(string); ok && str == name {
				return zones
			}
		}
		return append(zones, name)
	}

	// Need to add nested structure
	for i, z := range zones {
		if m, ok := z.(map[string]interface{}); ok {
			if sub, exists := m[name]; exists {
				if subSlice, ok := sub.([]interface{}); ok {
					m[name] = addZonePath(subSlice, remaining)
					return zones
				}
			}
		}
		if str, ok := z.(string); ok && str == name {
			// Convert string to map with nested content
			zones[i] = map[string]interface{}{
				name: addZonePath(nil, remaining),
			}
			return zones
		}
	}

	// Create new nested structure
	newMap := map[string]interface{}{
		name: addZonePath(nil, remaining),
	}
	return append(zones, newMap)
}

// removeZonePath removes a zone path from the nested structure
func removeZonePath(zones []interface{}, path []string) ([]interface{}, bool) {
	if len(path) == 0 {
		return zones, false
	}

	name := path[0]
	remaining := path[1:]

	for i, z := range zones {
		// Check string match
		if str, ok := z.(string); ok && str == name && len(remaining) == 0 {
			return append(zones[:i], zones[i+1:]...), true
		}

		// Check map match
		if m, ok := z.(map[string]interface{}); ok {
			if sub, exists := m[name]; exists {
				if len(remaining) == 0 {
					// Remove the entire map entry
					delete(m, name)
					if len(m) == 0 {
						return append(zones[:i], zones[i+1:]...), true
					}
					return zones, true
				}
				if subSlice, ok := sub.([]interface{}); ok {
					newSub, removed := removeZonePath(subSlice, remaining)
					if removed {
						m[name] = newSub
						return zones, true
					}
				}
			}
		}
	}

	return zones, false
}

// zoneToTree converts zone structure to a displayable tree
func zoneToTree(zones []interface{}) interface{} {
	if len(zones) == 0 {
		return nil
	}

	result := make(map[string]interface{})
	for _, z := range zones {
		switch item := z.(type) {
		case string:
			result[item] = nil
		case map[string]interface{}:
			for name, sub := range item {
				if subSlice, ok := sub.([]interface{}); ok {
					result[name] = zoneToTree(subSlice)
				} else {
					result[name] = nil
				}
			}
		}
	}
	return result
}

// LoadNodes loads all nodes from platforms/<platform>/nodes/ directory
func LoadNodes(dir, platform string) ([]Node, error) {
	var nodes []Node

	platformsDir := filepath.Join(dir, "platforms")
	if platform != "" {
		// Load from specific platform
		nodes, err := loadNodesFromPlatform(platformsDir, platform)
		if err != nil {
			return nil, err
		}
		return nodes, nil
	}

	// Load from all platforms
	entries, err := os.ReadDir(platformsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read platforms directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		platformNodes, err := loadNodesFromPlatform(platformsDir, entry.Name())
		if err != nil {
			continue // Skip platforms with errors
		}
		nodes = append(nodes, platformNodes...)
	}

	return nodes, nil
}

func loadNodesFromPlatform(platformsDir, platform string) ([]Node, error) {
	nodesDir := filepath.Join(platformsDir, platform, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var nodes []Node
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(nodesDir, entry.Name()))
		if err != nil {
			continue
		}

		var node Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			continue
		}
		node.Hostname = strings.TrimSuffix(entry.Name(), ".yaml")
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// NodesForZone returns nodes allocated to a zone path or its children
func NodesForZone(nodes []Node, zonePath string) []Node {
	var result []Node
	for _, node := range nodes {
		for _, c := range node.Zones {
			// Match exact zone path or children
			if c == zonePath || strings.HasPrefix(c, zonePath+".") {
				result = append(result, node)
				break
			}
		}
	}
	return result
}

// LoadNodesByPlatform groups nodes by their platform
func LoadNodesByPlatform(dir string) (map[string][]Node, error) {
	result := make(map[string][]Node)

	platformsDir := filepath.Join(dir, "platforms")
	entries, err := os.ReadDir(platformsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("failed to read platforms directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		nodes, err := loadNodesFromPlatform(platformsDir, entry.Name())
		if err != nil {
			continue
		}
		if len(nodes) > 0 {
			result[entry.Name()] = nodes
		}
	}

	return result, nil
}

// Rename renames a zone path preserving YAML order
func (t *Topology) Rename(oldPath, newPath string) error {
	oldParts := strings.Split(oldPath, ".")
	newParts := strings.Split(newPath, ".")

	if len(oldParts) != len(newParts) {
		return fmt.Errorf("old and new paths must have the same depth")
	}

	// Find the differing segment (should be exactly one for a rename)
	diffIdx := -1
	for i := 0; i < len(oldParts); i++ {
		if oldParts[i] != newParts[i] {
			if diffIdx != -1 {
				return fmt.Errorf("rename can only change one segment at a time")
			}
			diffIdx = i
		}
	}

	if diffIdx == -1 {
		return fmt.Errorf("old and new paths are identical")
	}

	// Update yaml.Node
	node := t.YAMLNode()
	if node != nil && len(node.Content) > 0 {
		renameInNode(node.Content[0], oldParts, newParts, diffIdx, 0)
	}

	// Update data for consistency
	t.updateDataForRename(oldParts, newParts, diffIdx)

	return nil
}

// renameInNode recursively finds and renames the target segment in yaml.Node
func renameInNode(node *yaml.Node, oldParts, newParts []string, diffIdx, depth int) bool {
	if node == nil || depth >= len(oldParts) {
		return false
	}

	target := oldParts[depth]
	newName := newParts[depth]

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == target {
				if depth == diffIdx {
					// This is the segment to rename
					key.Value = newName
					return true
				}
				// Continue deeper
				return renameInNode(value, oldParts, newParts, diffIdx, depth+1)
			}
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode && item.Value == target {
				if depth == diffIdx {
					item.Value = newName
					return true
				}
			} else if item.Kind == yaml.MappingNode {
				for j := 0; j < len(item.Content); j += 2 {
					if item.Content[j].Value == target {
						if depth == diffIdx {
							item.Content[j].Value = newName
							return true
						}
						return renameInNode(item.Content[j+1], oldParts, newParts, diffIdx, depth+1)
					}
				}
			}
		}
	}

	return false
}

// updateDataForRename updates data after a rename
func (t *Topology) updateDataForRename(oldParts, newParts []string, diffIdx int) {
	d := t.RawData()
	if d == nil {
		return
	}

	switch diffIdx {
	case 0:
		// Renaming root key
		if data, exists := d[oldParts[0]]; exists {
			d[newParts[0]] = data
			delete(d, oldParts[0])
		}
	case 1:
		// Renaming layer key
		root := oldParts[0]
		if d[root] != nil {
			if data, exists := d[root][oldParts[1]]; exists {
				d[root][newParts[1]] = data
				delete(d[root], oldParts[1])
			}
		}
	default:
		// Renaming within nested topology structure
		root := oldParts[0]
		layer := oldParts[1]
		if d[root] != nil && d[root][layer] != nil {
			d[root][layer] = renameInTopologyData(d[root][layer], oldParts[2:], newParts[2:], diffIdx-2)
		}
	}
}

// renameInTopologyData renames a path segment within the topology data structure
func renameInTopologyData(zones []interface{}, oldPath, newPath []string, diffIdx int) []interface{} {
	if len(oldPath) == 0 || diffIdx < 0 {
		return zones
	}

	target := oldPath[0]
	newName := newPath[0]

	for i, item := range zones {
		switch v := item.(type) {
		case string:
			if v == target && diffIdx == 0 {
				// Rename this string entry
				zones[i] = newName
				return zones
			}
		case map[string]interface{}:
			if sub, exists := v[target]; exists {
				if diffIdx == 0 {
					// Rename the key in this map
					v[newName] = sub
					delete(v, target)
					return zones
				}
				// Recurse deeper
				if subSlice, ok := sub.([]interface{}); ok {
					v[target] = renameInTopologyData(subSlice, oldPath[1:], newPath[1:], diffIdx-1)
					return zones
				}
			}
		}
	}

	return zones
}
