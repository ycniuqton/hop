package cli

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/cache"
	"github.com/ycniuqton/hop/internal/complete"
	"github.com/ycniuqton/hop/internal/selfinstall"
	"github.com/ycniuqton/hop/internal/skill"
	"github.com/ycniuqton/hop/internal/ui"
)

// UninstallCommand removes hop's files from this machine; the bucket is never touched.
type UninstallCommand struct {
	env        *Env
	yes        bool
	keepConfig bool
}

func (c *UninstallCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "uninstall", Short: "Remove hop and its local files from this machine", GroupID: string(GroupSetup)}
	cmd.Flags().BoolVarP(&c.yes, "yes", "y", false, "do not ask for confirmation")
	cmd.Flags().BoolVar(&c.keepConfig, "keep-config", false, "keep config.json (the workspace credentials)")
	return bindHandler(cmd, noArgs, c)
}

func (c *UninstallCommand) Run(context.Context, []string) error {
	fp, err := c.footprint()
	if err != nil {
		return err
	}
	items := fp.Existing()
	if len(items) == 0 {
		return c.env.Console.Done(ui.Outcome{Action: ui.Actions.Uninstall, Data: map[string]any{"removed": items}}, "nothing to remove")
	}
	if ok, err := c.confirm(items); err != nil || !ok {
		return err
	}
	device := ""
	if cfg, err := c.env.ConfigStore().Load(); err == nil {
		device = cfg.Device
	}
	removed, rmErr := fp.Remove(items)
	if err := c.env.Console.Done(ui.Outcome{Action: ui.Actions.Uninstall, Data: map[string]any{"removed": removed, "device": device}},
		"removed %d item(s); hop is uninstalled", len(removed)); err != nil || rmErr != nil {
		return errors.Join(rmErr, err)
	}
	if device != "" && !c.keepConfig {
		c.env.Console.Printf("Device %q is still listed in the workspace — run 'hop remove %s' on another device.", device, device)
	}
	return nil
}

func (c *UninstallCommand) confirm(items []selfinstall.Item) (bool, error) {
	c.env.Console.Printf("hop will remove:")
	for _, it := range items {
		c.env.Console.Printf("  %-10s %s", it.Kind, it.Path)
	}
	if c.yes {
		return true, nil
	}
	return c.env.Console.Confirm("Uninstall hop?", false)
}

// footprint lists every place hop writes: binary, completions, skill, cache and (unless kept) config.
func (c *UninstallCommand) footprint() (selfinstall.Footprint, error) {
	bin, err := selfinstall.Locate(c.env.Executable)
	if err != nil {
		return selfinstall.Footprint{}, err
	}
	home := c.env.home()
	items := []selfinstall.Item{{Kind: selfinstall.KindBinary, Path: bin.Path}}
	for _, sh := range complete.Shells() {
		_, inst, _ := complete.Lookup(sh)
		items = append(items, selfinstall.Item{Kind: selfinstall.KindCompletion, Path: inst.Path(home)})
	}
	items = append(items,
		selfinstall.Item{Kind: selfinstall.KindSkill, Path: skill.Targets[skill.ScopeUser].Path(home, c.env.Getenv(EnvVars.ClaudeConfig), "")},
		selfinstall.Item{Kind: selfinstall.KindCache, Path: filepath.Join(c.env.CacheRoot(), cache.Policy.AppDir), Dir: true})
	if !c.keepConfig {
		items = append(items, selfinstall.Item{Kind: selfinstall.KindConfig, Path: c.env.ConfigStore().Path})
	}
	return selfinstall.Footprint{Items: items}, nil
}
