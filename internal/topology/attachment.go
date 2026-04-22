package topology

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Attachment represents a component attached to a zone path
type Attachment struct {
	Component string
	Playbook  string
	Zone      string
}

// LoadAttachments scans playbooks for component attachments to a zone path
func LoadAttachments(dir, zonePath string) ([]Attachment, error) {
	var attachments []Attachment

	// Scan src/<layer>/<layer>.yaml playbooks
	srcDir := filepath.Join(dir, "src")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		playbookPath := filepath.Join(srcDir, entry.Name(), entry.Name()+".yaml")
		data, err := os.ReadFile(playbookPath)
		if err != nil {
			continue
		}

		// Parse playbook - roles can be strings or dicts with "role" key
		var plays []struct {
			Hosts string        `yaml:"hosts"`
			Roles []interface{} `yaml:"roles"`
		}
		if err := yaml.Unmarshal(data, &plays); err != nil {
			continue
		}

		for _, play := range plays {
			// Match exact zone path or children
			if play.Hosts == zonePath || strings.HasPrefix(play.Hosts, zonePath+".") {
				for _, r := range play.Roles {
					var roleName string
					switch role := r.(type) {
					case string:
						// Simple string: "- foundation.applications.os"
						roleName = role
					case map[string]interface{}:
						// Dict with role key: "- role: foundation.applications.cluster"
						if name, ok := role["role"].(string); ok {
							roleName = name
						}
					}
					if roleName != "" {
						attachments = append(attachments, Attachment{
							Component: roleName,
							Playbook:  playbookPath,
							Zone:      play.Hosts,
						})
					}
				}
			}
		}
	}

	return attachments, nil
}

// HasAttachments checks if a zone path has any component attachments
func HasAttachments(dir, zonePath string) (bool, []Attachment, error) {
	attachments, err := LoadAttachments(dir, zonePath)
	if err != nil {
		return false, nil, err
	}
	return len(attachments) > 0, attachments, nil
}

// UpdateAttachments renames zone path references in all playbooks
func UpdateAttachments(dir, oldZone, newZone string) ([]string, error) {
	var updatedFiles []string

	srcDir := filepath.Join(dir, "src")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		playbookPath := filepath.Join(srcDir, entry.Name(), entry.Name()+".yaml")
		data, err := os.ReadFile(playbookPath)
		if err != nil {
			continue
		}

		// Parse as yaml.Node to preserve formatting
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}

		updated := updateHostsInNode(&doc, oldZone, newZone)
		if updated {
			newData, err := yaml.Marshal(&doc)
			if err != nil {
				continue
			}
			if err := os.WriteFile(playbookPath, newData, 0644); err != nil {
				continue
			}
			updatedFiles = append(updatedFiles, playbookPath)
		}
	}

	return updatedFiles, nil
}

// updateHostsInNode recursively updates hosts fields in a yaml.Node
func updateHostsInNode(node *yaml.Node, oldZone, newZone string) bool {
	updated := false

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if updateHostsInNode(child, oldZone, newZone) {
				updated = true
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if updateHostsInNode(child, oldZone, newZone) {
				updated = true
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == "hosts" && value.Kind == yaml.ScalarNode {
				// Check for exact match or prefix match
				if value.Value == oldZone {
					value.Value = newZone
					updated = true
				} else if strings.HasPrefix(value.Value, oldZone+".") {
					value.Value = newZone + value.Value[len(oldZone):]
					updated = true
				}
			} else {
				if updateHostsInNode(value, oldZone, newZone) {
					updated = true
				}
			}
		}
	}

	return updated
}

// UpdateAllocations renames zone path references in all node files
func UpdateAllocations(dir, oldZone, newZone string) ([]string, error) {
	var updatedFiles []string

	platformsDir := filepath.Join(dir, "platforms")
	platforms, err := os.ReadDir(platformsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, platform := range platforms {
		if !platform.IsDir() {
			continue
		}

		nodesDir := filepath.Join(platformsDir, platform.Name(), "nodes")
		nodeFiles, err := os.ReadDir(nodesDir)
		if err != nil {
			continue
		}

		for _, nodeFile := range nodeFiles {
			if nodeFile.IsDir() || !strings.HasSuffix(nodeFile.Name(), ".yaml") {
				continue
			}

			nodePath := filepath.Join(nodesDir, nodeFile.Name())
			data, err := os.ReadFile(nodePath)
			if err != nil {
				continue
			}

			// Parse as yaml.Node to preserve formatting
			var doc yaml.Node
			if err := yaml.Unmarshal(data, &doc); err != nil {
				continue
			}

			updated := updateZoneInNode(&doc, oldZone, newZone)
			if updated {
				newData, err := yaml.Marshal(&doc)
				if err != nil {
					continue
				}
				if err := os.WriteFile(nodePath, newData, 0644); err != nil {
					continue
				}
				updatedFiles = append(updatedFiles, nodePath)
			}
		}
	}

	return updatedFiles, nil
}

// updateZoneInNode updates zones array entries in a yaml.Node
func updateZoneInNode(node *yaml.Node, oldZone, newZone string) bool {
	updated := false

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if updateZoneInNode(child, oldZone, newZone) {
				updated = true
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == "zones" && value.Kind == yaml.SequenceNode {
				// Update zones array entries
				for _, item := range value.Content {
					if item.Kind == yaml.ScalarNode {
						if item.Value == oldZone {
							item.Value = newZone
							updated = true
						} else if strings.HasPrefix(item.Value, oldZone+".") {
							item.Value = newZone + item.Value[len(oldZone):]
							updated = true
						}
					}
				}
			} else {
				if updateZoneInNode(value, oldZone, newZone) {
					updated = true
				}
			}
		}
	}

	return updated
}
