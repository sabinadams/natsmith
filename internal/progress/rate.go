package progress

import (
	"fmt"
	"strings"
	"time"
)

// FormatElapsed renders a duration for command footers.
func FormatElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	d -= time.Duration(h) * time.Hour
	m := int(d / time.Minute)
	d -= time.Duration(m) * time.Minute
	s := int(d / time.Second)

	switch {
	case h > 0:
		if s > 0 {
			return fmt.Sprintf("%dh %dm %ds", h, m, s)
		}
		if m > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	case m > 0:
		if s > 0 {
			return fmt.Sprintf("%dm %ds", m, s)
		}
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// BucketStats holds optional per-bucket counters for throughput display.
type BucketStats struct {
	Items int64
	Bytes int64
}

func formatBucketSuffix(elapsed time.Duration, stats BucketStats) string {
	if elapsed <= 0 {
		return ""
	}

	parts := []string{FormatElapsed(elapsed)}
	if stats.Bytes > 0 && elapsed >= time.Second {
		parts = append(parts, humanizeBytes(int64(float64(stats.Bytes)/elapsed.Seconds()))+"/s")
	} else if stats.Items > 0 && elapsed >= time.Second {
		rate := float64(stats.Items) / elapsed.Seconds()
		switch {
		case rate >= 1_000_000:
			parts = append(parts, fmt.Sprintf("%.1fM/s", rate/1_000_000))
		case rate >= 1_000:
			parts = append(parts, fmt.Sprintf("%.1fK/s", rate/1_000))
		default:
			parts = append(parts, fmt.Sprintf("%.0f/s", rate))
		}
	}
	return " — " + strings.Join(parts, ", ")
}
