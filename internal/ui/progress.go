package ui

import (
	"io"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ProgressPolicy decides when a bar is shown (spec section 8).
var ProgressPolicy = struct {
	MinBytes int64
	Throttle time.Duration
}{MinBytes: 1 << 20, Throttle: 100 * time.Millisecond}

// Progress tracks one transfer; a nil bar means progress is hidden.
type Progress struct {
	bar *progressbar.ProgressBar
}

// Progress starts a bar on stderr for large transfers on a TTY, never with -q or --json.
func (c *Console) Progress(label string, total int64) *Progress {
	if c.Quiet || c.JSON || !c.ErrTTY || total < ProgressPolicy.MinBytes {
		return &Progress{}
	}
	return &Progress{bar: progressbar.NewOptions64(total,
		progressbar.OptionSetWriter(c.Err),
		progressbar.OptionSetDescription(label),
		progressbar.OptionShowBytes(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionThrottle(ProgressPolicy.Throttle),
	)}
}

// Reader counts bytes read through r.
func (p *Progress) Reader(r io.Reader) io.Reader {
	if p.bar == nil {
		return r
	}
	return io.TeeReader(r, p.bar)
}

// WriterAt counts bytes written through w.
func (p *Progress) WriterAt(w io.WriterAt) io.WriterAt {
	if p.bar == nil {
		return w
	}
	return countingWriterAt{w: w, bar: p.bar}
}

// Done clears the bar.
func (p *Progress) Done() {
	if p.bar != nil {
		_ = p.bar.Finish()
	}
}

type countingWriterAt struct {
	w   io.WriterAt
	bar *progressbar.ProgressBar
}

func (c countingWriterAt) WriteAt(b []byte, off int64) (int, error) {
	n, err := c.w.WriteAt(b, off)
	_ = c.bar.Add(n)
	return n, err
}
