package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/target"
)

// Direction is which way one copy job moves bytes.
type Direction int

const (
	Upload Direction = iota
	Download
	RemoteCopy
)

// CopyOptions are the cp flags.
type CopyOptions struct {
	Recursive bool
	Force     bool
}

// CopyResult summarizes a finished copy.
type CopyResult struct {
	Files int
	Bytes int64
	jobs  []copyJob
}

type copyJob struct {
	dir      Direction
	local    LocalFile
	srcKey   string
	dstKey   string
	dstLocal string
	dstRef   string
	size     int64
}

// copyStrategy executes one direction; strategies are selected by Direction, never by type switch.
type copyStrategy interface {
	exists(ctx context.Context, j copyJob) (bool, error)
	run(ctx context.Context, j copyJob, force bool) error
}

func (m *StoreManager) strategies() map[Direction]copyStrategy {
	return map[Direction]copyStrategy{
		Upload:     uploadStrategy{ws: m.ws, t: m.transfers},
		Download:   downloadStrategy{t: m.transfers},
		RemoteCopy: remoteCopyStrategy{ws: m.ws},
	}
}

type uploadStrategy struct {
	ws *Workspace
	t  *Transfers
}

func (s uploadStrategy) exists(ctx context.Context, j copyJob) (bool, error) {
	return remoteExists(ctx, s.ws, j.dstKey)
}

func (s uploadStrategy) run(ctx context.Context, j copyJob, _ bool) error {
	return s.t.Upload(ctx, j.local, j.dstKey)
}

type downloadStrategy struct {
	t *Transfers
}

func (downloadStrategy) exists(_ context.Context, j copyJob) (bool, error) {
	_, err := os.Lstat(j.dstLocal)
	return err == nil, nil
}

func (s downloadStrategy) run(ctx context.Context, j copyJob, force bool) error {
	_, err := s.t.Download(ctx, j.srcKey, j.dstLocal, force)
	return err
}

type remoteCopyStrategy struct {
	ws *Workspace
}

func (s remoteCopyStrategy) exists(ctx context.Context, j copyJob) (bool, error) {
	return remoteExists(ctx, s.ws, j.dstKey)
}

func (s remoteCopyStrategy) run(ctx context.Context, j copyJob, _ bool) error {
	return s.ws.Storage.Copy(ctx, j.srcKey, j.dstKey)
}

func remoteExists(ctx context.Context, ws *Workspace, key string) (bool, error) {
	_, err := ws.Storage.Head(ctx, key)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Copy plans and runs uploads, downloads and server-side copies (spec section 5 rules).
func (m *StoreManager) Copy(ctx context.Context, srcs []target.Target, dst target.Target, opt CopyOptions) (CopyResult, error) {
	if err := m.resolveDevices(ctx, append(append([]target.Target{}, srcs...), dst)); err != nil {
		return CopyResult{}, err
	}
	jobs, err := (&copyPlanner{m: m, opt: opt}).plan(ctx, srcs, dst)
	if err != nil {
		return CopyResult{}, err
	}
	strategies := m.strategies()
	if !opt.Force {
		if err := m.preflight(ctx, jobs, strategies); err != nil {
			return CopyResult{}, err
		}
	}
	err = Pool{Workers: Concurrency.Transfers}.Run(ctx, len(jobs), func(ctx context.Context, i int) error {
		return strategies[jobs[i].dir].run(ctx, jobs[i], opt.Force)
	})
	res := CopyResult{Files: len(jobs), jobs: jobs}
	for _, j := range jobs {
		res.Bytes += j.size
	}
	return res, err
}

func (m *StoreManager) preflight(ctx context.Context, jobs []copyJob, strategies map[Direction]copyStrategy) error {
	found := make([]bool, len(jobs))
	err := Pool{Workers: Concurrency.Lookups}.Run(ctx, len(jobs), func(ctx context.Context, i int) error {
		ok, err := strategies[jobs[i].dir].exists(ctx, jobs[i])
		found[i] = ok
		return err
	})
	if err != nil {
		return err
	}
	for i, ok := range found {
		if ok {
			return apperr.New(apperr.ExitConflict, "exists: %s (use -f to overwrite)", jobs[i].dstRef)
		}
	}
	return nil
}

// copyPlanner expands sources into concrete jobs using cp/scp destination rules.
type copyPlanner struct {
	m       *StoreManager
	opt     CopyOptions
	dst     target.Target
	dstDir  bool
	dstRoot string
}

func (p *copyPlanner) plan(ctx context.Context, srcs []target.Target, dst target.Target) ([]copyJob, error) {
	p.dst, p.dstDir = dst, p.isDirDest(dst)
	if len(srcs) > 1 && !p.dstDir {
		return nil, apperr.New(apperr.ExitUsage, "copying several sources needs a folder destination").
			WithHint("end the destination with /")
	}
	var jobs []copyJob
	for _, src := range srcs {
		if src.Kind == target.Local && dst.Kind == target.Local {
			return nil, apperr.New(apperr.ExitUsage, "use the regular cp for local copies")
		}
		more, err := p.expand(ctx, src)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, more...)
	}
	return jobs, nil
}

func (p *copyPlanner) isDirDest(dst target.Target) bool {
	if dst.Kind != target.Local {
		return dst.Dir
	}
	info, err := os.Stat(dst.Path)
	return dst.Dir || (err == nil && info.IsDir())
}

func (p *copyPlanner) expand(ctx context.Context, src target.Target) ([]copyJob, error) {
	if src.Kind == target.Local {
		return p.expandLocal(src)
	}
	return p.expandRemote(ctx, src)
}

func (p *copyPlanner) expandLocal(src target.Target) ([]copyJob, error) {
	f, err := p.m.transfers.OpenLocal(src.Path)
	if err != nil {
		return nil, err
	}
	base := filepath.Base(filepath.Clean(src.Path))
	if !f.Mode.IsDir() {
		return []copyJob{p.uploadJob(f, p.fileDest(base))}, nil
	}
	if !p.opt.Recursive {
		return nil, apperr.New(apperr.ExitUsage, "%s is a directory", src.Path).WithHint("use -r")
	}
	var jobs []copyJob
	err = filepath.WalkDir(src.Path, func(fp string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, _ := filepath.Rel(src.Path, fp)
		lf, err := p.m.transfers.OpenLocal(fp)
		jobs = append(jobs, p.uploadJob(lf, p.treeDest(base, filepath.ToSlash(rel))))
		return err
	})
	return jobs, err
}

func (p *copyPlanner) uploadJob(f LocalFile, part string) copyJob {
	sub := path.Join(p.dst.Path, part)
	key := p.m.ws.Layout.StoreKey(p.dst.Device, sub)
	return copyJob{dir: Upload, local: f, dstKey: key, dstRef: RemoteRef(p.dst.Device, sub), size: f.Size}
}

// fileDest is the part below the destination for a single file: its base name when copying into a folder.
func (p *copyPlanner) fileDest(base string) string {
	if p.dstDir {
		return base
	}
	return ""
}

// treeDest is the part below the destination for a file inside a copied folder.
func (p *copyPlanner) treeDest(base, rel string) string {
	if p.dstDir {
		return path.Join(base, rel)
	}
	return rel
}

func (p *copyPlanner) expandRemote(ctx context.Context, src target.Target) ([]copyJob, error) {
	isTree, size, err := p.m.classify(ctx, src)
	if err != nil {
		return nil, err
	}
	srcKey := p.m.ws.Layout.StoreKey(src.Device, src.Path)
	if !isTree {
		return p.remoteJobs([]string{srcKey}, []string{p.fileDest(path.Base(src.Path))}, []int64{size})
	}
	if !p.opt.Recursive {
		return nil, apperr.New(apperr.ExitUsage, "%s is a folder", src.Raw).WithHint("use -r")
	}
	prefix := p.m.ws.Layout.DirPrefix(src.Device, src.Path)
	objs, err := p.m.ws.listAll(ctx, prefix)
	if err != nil {
		return nil, err
	}
	base := path.Base(src.Path)
	if src.Path == "" {
		base = ""
	}
	var keys, subs []string
	var sizes []int64
	for _, o := range objs {
		if !strings.HasSuffix(o.Key, Paths.Sep) {
			keys, sizes = append(keys, o.Key), append(sizes, o.Size)
			subs = append(subs, p.treeDest(base, strings.TrimPrefix(o.Key, prefix)))
		}
	}
	return p.remoteJobs(keys, subs, sizes)
}

func (p *copyPlanner) remoteJobs(keys, parts []string, sizes []int64) ([]copyJob, error) {
	jobs := make([]copyJob, len(keys))
	for i := range keys {
		dest, err := p.destination(parts[i])
		if err != nil {
			return nil, err
		}
		j := copyJob{dir: RemoteCopy, srcKey: keys[i], size: sizes[i], dstRef: dest}
		if p.dst.Kind == target.Local {
			j.dir, j.dstLocal = Download, dest
		} else {
			sub := path.Join(p.dst.Path, parts[i])
			j.dstKey, j.dstRef = p.m.ws.Layout.StoreKey(p.dst.Device, sub), RemoteRef(p.dst.Device, sub)
		}
		jobs[i] = j
	}
	return jobs, nil
}

// destination maps a part below the destination and refuses parts that escape it.
func (p *copyPlanner) destination(part string) (string, error) {
	clean := path.Clean("/" + part)
	if part != "" && strings.TrimPrefix(clean, "/") != strings.TrimPrefix(path.Clean(part), "./") {
		return "", apperr.New(apperr.ExitUsage, "refusing to write outside the destination: %s", part)
	}
	if p.dst.Kind != target.Local {
		return "", nil
	}
	if part == "" {
		return p.dst.Path, nil
	}
	return filepath.Join(p.dst.Path, filepath.FromSlash(strings.TrimPrefix(clean, "/"))), nil
}
