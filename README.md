# plasmactl-topology

A [Launchr](https://github.com/launchrctl/launchr) plugin for [Plasmactl](https://github.com/plasmash/plasmactl) that manages the topology structure for Plasma platforms.

## Overview

`plasmactl-topology` manages the platform's "skeleton" - the structural framework where applications and agents attach. The topology maps logical architecture to physical resources, ensuring components get appropriate compute resources (GPU for AI, storage for data, etc.).

## Features

- **Topology Structure**: Hierarchical definition of platform zones
- **Zone Management**: Add and remove zones
- **Node Visibility**: See which nodes are allocated to each zone
- **Component Visibility**: See which components are attached to each zone

## Concepts

### Topology Structure

The topology is defined in `topology.yaml` as a hierarchical structure:

```yaml
platform:
  foundation:
    - cluster:
      - control
      - nodes
    - storage:
      - kv
    - network:
      - ingress
  interaction:
    - observability
    - management
  cognition:
    - data
    - knowledge
```

Each path (e.g., `platform.foundation.cluster.control`) represents a zone that:
- Can have nodes allocated to it
- Can have components attached to it
- Can have specific configuration in group_vars

## Commands

### zone:list

List zones from `topology.yaml`:

```bash
# List all zones (flat)
plasmactl zone:list

# List as tree
plasmactl zone:list --tree

# List zone and its children
plasmactl zone:list platform.interaction
plasmactl zone:list platform.foundation.cluster --tree
```

Options:
- `-t, --tree`: Show as tree instead of flat list

### zone:show

Show details for a zone:

```bash
# Show zone details
plasmactl zone:show platform.interaction.observability

# Filter nodes by platform
plasmactl zone:show platform.foundation.cluster.control --platform dev
```

Options:
- `-p, --platform`: Filter nodes by platform instance (default: all)

Output includes:
- Allocated nodes (from `inst/<platform>/nodes/`)
- Attached components (from layer playbooks)

### zone:add

Add a new zone:

```bash
plasmactl zone:add platform.interaction.analytics
plasmactl zone:add platform.cognition.ml.training
```

### zone:remove

Remove a zone:

```bash
plasmactl zone:remove platform.interaction.legacy
```

**Safety**: Fails if nodes are allocated or components are attached. Use `node:allocate` and `component:detach` first to clean up.

## Project Structure

```
plasmactl-topology/
├── plugin.go                        # Plugin registration
├── actions/
│   ├── add/
│   │   ├── add.yaml                 # Action definition
│   │   └── add.go                   # Implementation
│   ├── list/
│   │   ├── list.yaml
│   │   └── list.go
│   ├── remove/
│   │   ├── remove.yaml
│   │   └── remove.go
│   └── show/
│       ├── show.yaml
│       └── show.go
└── internal/
    └── topology/                    # Topology operations
        └── topology.go
```

## Directory Structure

The topology interacts with several locations:

```
.
├── topology.yaml                   # Topology definition
├── inst/
│   └── <platform>/
│       └── nodes/
│           └── <hostname>.yaml     # Node allocations
└── src/
    └── <layer>/
        ├── <layer>.yaml            # Component attachments (playbooks)
        └── cfg/
            └── <zone>/             # Zone-specific configuration
                ├── vars.yaml
                └── vault.yaml
```

## Workflow Example

```bash
# 1. View current topology structure
plasmactl zone:list --tree

# 2. Add a new zone for analytics
plasmactl zone:add platform.interaction.analytics

# 3. Allocate nodes to the new zone
plasmactl node:allocate node001 platform.interaction.analytics

# 4. Attach a component to the zone
plasmactl component:attach interaction.applications.analytics platform.interaction.analytics

# 5. Verify the setup
plasmactl zone:show platform.interaction.analytics

# 6. Remove a legacy zone (after cleanup)
plasmactl node:allocate node001 platform.interaction.legacy-
plasmactl component:detach interaction.applications.old platform.interaction.legacy
plasmactl zone:remove platform.interaction.legacy
```

## Related Commands

| Plugin | Command | Purpose |
|--------|---------|---------|
| plasmactl-node | `node:allocate` | Allocate nodes to zones |
| plasmactl-component | `component:attach` | Attach components to zones |
| plasmactl-component | `component:detach` | Detach components from zones |
| plasmactl-platform | `platform:deploy` | Deploy to zones |

## Documentation

- [Plasmactl](https://github.com/plasmash/plasmactl) - Main CLI tool
- [plasmactl-node](https://github.com/plasmash/plasmactl-node) - Node management
- [plasmactl-component](https://github.com/plasmash/plasmactl-component) - Component management
- [Plasma Platform](https://plasma.sh) - Platform documentation

## License

[European Union Public License 1.2 (EUPL-1.2)](LICENSE)
