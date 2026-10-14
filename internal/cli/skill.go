package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/skill"
	"github.com/ycniuqton/hop/internal/ui"
)

// SkillCommand groups the coding-agent skill subcommands.
type SkillCommand struct{}

func (c *SkillCommand) Build(env *Env) *cobra.Command {
	cmd := &cobra.Command{Use: "skill", Short: "Teach coding agents (Claude Code, Codex, …) to use hop", GroupID: string(GroupSetup)}
	cmd.AddCommand((&SkillInstallCommand{}).Build(env), (&SkillPrintCommand{}).Build(env))
	return cmd
}

// SkillInstallCommand writes the Claude Code skill for this user or this project.
type SkillInstallCommand struct {
	env     *Env
	project bool
}

func (c *SkillInstallCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "install", Short: "Install the hop skill for Claude Code (~/.claude/skills/hop)"}
	cmd.Flags().BoolVar(&c.project, "project", false, "install into ./.claude/skills/hop for this project instead")
	return bindHandler(cmd, noArgs, c)
}

func (c *SkillInstallCommand) Run(context.Context, []string) error {
	scope := skill.ScopeUser
	if c.project {
		scope = skill.ScopeProject
	}
	wd, err := os.Getwd()
	if err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot read the working directory: %v", err)
	}
	path := skill.Targets[scope].Path(c.env.home(), c.env.Getenv(EnvVars.ClaudeConfig), wd)
	if err := skill.Install(path); err != nil {
		return err
	}
	return c.env.Console.Done(ui.Outcome{Action: ui.Actions.InstallSkill, Data: map[string]any{"path": path, "scope": scope}},
		"installed %s — start a new Claude Code session to use it", path)
}

// SkillPrintCommand prints the guidance as plain Markdown for AGENTS.md and other agents.
type SkillPrintCommand struct {
	env *Env
}

func (c *SkillPrintCommand) Build(env *Env) *cobra.Command {
	c.env = env
	return bindHandler(&cobra.Command{Use: "print", Short: "Print the guidance as Markdown (for AGENTS.md and other agents)"}, noArgs, c)
}

func (c *SkillPrintCommand) Run(context.Context, []string) error {
	_, err := fmt.Fprint(c.env.Console.Out, skill.Guidance())
	return err
}
