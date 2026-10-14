package ui

import (
	"fmt"
	"time"
)

// Units are the human size steps (binary multiples, short labels).
var Units = struct {
	Step   float64
	Labels []string
}{Step: 1024, Labels: []string{"B", "KB", "MB", "GB", "TB", "PB"}}

// Size formats a byte count like "12 MB".
func Size(n int64) string {
	v, i := float64(n), 0
	for v >= Units.Step && i < len(Units.Labels)-1 {
		v /= Units.Step
		i++
	}
	if i == 0 || v >= 10 {
		return fmt.Sprintf("%.0f %s", v, Units.Labels[i])
	}
	return fmt.Sprintf("%.1f %s", v, Units.Labels[i])
}

// Ago formats a past time relative to now, like "2m ago".
func Ago(now, t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", max(0, int(d.Seconds())))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
