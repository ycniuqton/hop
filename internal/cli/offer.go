package cli

import (
	"github.com/ycniuqton/hop/internal/complete"
)

// completionOffer asks once, after init or join, whether to install tab completion.
type completionOffer struct {
	env *Env
}

func (o completionOffer) Offer() {
	c := o.env.Console
	if !c.CanPrompt() || c.JSON {
		return
	}
	sh, inst, err := complete.Detect(o.env.Getenv(EnvVars.Shell))
	if err != nil {
		return
	}
	ok, err := c.Confirm("Enable tab completion for "+string(sh)+"?", true)
	if err != nil || !ok {
		return
	}
	path, err := complete.Install(o.env.root, inst, o.env.home())
	if err != nil {
		c.Warnf("could not install completion: %v", err)
		return
	}
	c.Successf("Installed completion to %s — open a new shell to use it", path)
	if hint := inst.Hint(path, o.env.Getenv); hint != "" {
		c.Printf("%s", hint)
	}
}
