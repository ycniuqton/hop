package cli

import (
	"context"

	"github.com/ycniuqton/hop/internal/target"
	"github.com/ycniuqton/hop/internal/workspace"
)

// hotInvocation is a parsed hot command: the services, the resolved shelf and the remaining arguments.
type hotInvocation struct {
	svc    *workspace.Services
	shelf  workspace.Shelf
	others target.List
}

// hotArgs parses @ in any position and resolves the shelf (rules 5 and 6 of spec section 6).
type hotArgs struct {
	env         *Env
	allowRemote bool
}

func (h hotArgs) parse(ctx context.Context, args []string) (hotInvocation, error) {
	svc, err := h.env.Services(ctx)
	if err != nil {
		return hotInvocation{}, err
	}
	p, _ := h.env.Parser()
	list, err := p.ParseAll(args)
	if err != nil {
		return hotInvocation{}, err
	}
	parsed, err := list.ForHot(h.allowRemote)
	if err != nil {
		return hotInvocation{}, err
	}
	shelf, err := svc.Shelves.Resolve(ctx, parsed.Shelf)
	return hotInvocation{svc: svc, shelf: shelf, others: parsed.Others}, err
}
