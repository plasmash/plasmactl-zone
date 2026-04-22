// Package plasmactlzone implements a launchr plugin with zone (topology) management functionality
package plasmactlzone

import (
	"context"
	"embed"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"

	"github.com/plasmash/plasmactl-zone/actions/add"
	"github.com/plasmash/plasmactl-zone/actions/list"
	"github.com/plasmash/plasmactl-zone/actions/query"
	"github.com/plasmash/plasmactl-zone/actions/remove"
	"github.com/plasmash/plasmactl-zone/actions/rename"
	"github.com/plasmash/plasmactl-zone/actions/show"
)

//go:embed actions/*/*.yaml
var actionYamlFS embed.FS

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is [launchr.Plugin] plugin providing topology management functionality.
type Plugin struct {
	cfg launchr.Config
}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		Weight: 10,
	}
}

// OnAppInit implements [launchr.Plugin] interface.
func (p *Plugin) OnAppInit(app launchr.App) error {
	app.Services().Get(&p.cfg)
	return nil
}

// actionRunner is implemented by all topology action structs.
type actionRunner interface {
	SetLogger(*launchr.Logger)
	SetTerm(*launchr.Terminal)
	Execute() error
	Result() any
}

// createAction builds a launchr action from YAML and a factory function.
func createAction(yamlFile, name string, factory func(*action.Input) actionRunner) *action.Action {
	data, _ := actionYamlFS.ReadFile(yamlFile)
	act := action.NewFromYAML(name, data)
	act.SetRuntime(action.NewFnRuntimeWithResult(func(_ context.Context, a *action.Action) (any, error) {
		log, term := getLogger(a)
		runner := factory(a.Input())
		runner.SetLogger(log)
		runner.SetTerm(term)
		err := runner.Execute()
		return runner.Result(), err
	}))
	return act
}

// optString returns a string option value or empty string if nil.
func optString(input *action.Input, name string) string {
	if v := input.Opt(name); v != nil {
		return v.(string)
	}
	return ""
}

// optBool returns a bool option value or false if nil.
func optBool(input *action.Input, name string) bool {
	if v := input.Opt(name); v != nil {
		return v.(bool)
	}
	return false
}

// argString returns a string argument value or empty string if nil.
func argString(input *action.Input, name string) string {
	if v := input.Arg(name); v != nil {
		return v.(string)
	}
	return ""
}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(_ context.Context) ([]*action.Action, error) {
	return []*action.Action{
		createAction("actions/list/list.yaml", "zone:list", func(input *action.Input) actionRunner {
			return &list.List{
				Dir:  optString(input, "dir"),
				Zone: argString(input, "zone"),
				Tree: optBool(input, "tree"),
			}
		}),
		createAction("actions/show/show.yaml", "zone:show", func(input *action.Input) actionRunner {
			return &show.Show{
				Dir:      optString(input, "dir"),
				Zone:     argString(input, "zone"),
				Platform: optString(input, "platform"),
				Kind:     optString(input, "kind"),
			}
		}),
		createAction("actions/add/add.yaml", "zone:add", func(input *action.Input) actionRunner {
			return &add.Add{
				Dir:   optString(input, "dir"),
				Zone:  input.Arg("zone").(string),
				Force: optBool(input, "force"),
			}
		}),
		createAction("actions/remove/remove.yaml", "zone:remove", func(input *action.Input) actionRunner {
			return &remove.Remove{
				Dir:    optString(input, "dir"),
				Zone:   input.Arg("zone").(string),
				DryRun: optBool(input, "dry-run"),
			}
		}),
		createAction("actions/rename/rename.yaml", "zone:rename", func(input *action.Input) actionRunner {
			return &rename.Rename{
				Dir:    optString(input, "dir"),
				Old:    input.Arg("old").(string),
				New:    input.Arg("new").(string),
				DryRun: optBool(input, "dry-run"),
			}
		}),
		createAction("actions/query/query.yaml", "zone:query", func(input *action.Input) actionRunner {
			return &query.Query{
				Dir:        optString(input, "dir"),
				Identifier: input.Arg("identifier").(string),
				Kind:       optString(input, "kind"),
			}
		}),
	}, nil
}

func getLogger(a *action.Action) (*launchr.Logger, *launchr.Terminal) {
	log := launchr.Log()
	if rt, ok := a.Runtime().(action.RuntimeLoggerAware); ok {
		log = rt.LogWith()
	}

	term := launchr.Term()
	if rt, ok := a.Runtime().(action.RuntimeTermAware); ok {
		term = rt.Term()
	}

	return log, term
}
