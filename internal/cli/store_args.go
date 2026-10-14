package cli

import (
	"context"

	"github.com/ycniuqton/hop/internal/target"
	"github.com/ycniuqton/hop/internal/workspace"
)

// storeArgs parses store-command arguments and rejects bare @device targets (rule 7).
type storeArgs struct {
	env *Env
}

func (s storeArgs) parse(ctx context.Context, args []string) (*workspace.Services, target.List, error) {
	svc, err := s.env.Services(ctx)
	if err != nil {
		return nil, nil, err
	}
	p, _ := s.env.Parser()
	list, err := p.ParseAll(args)
	if err != nil {
		return nil, nil, err
	}
	list, err = list.ForStore()
	return svc, list, err
}
