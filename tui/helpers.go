package tui

import (
	"fmt"
	"strings"
	"time"
)

// These pad/truncate by rune count, not bytes, so a multi-byte glyph (e.g. the
// ▲/▼ sort marker) lines up with single-byte ASCII the same way on screen.

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func padRight(s string, n int) string {
	if r := []rune(s); len(r) < n {
		return s + spaces(n-len(r))
	}
	return trunc(s, n)
}

func padLeft(s string, n int) string {
	if r := []rune(s); len(r) < n {
		return spaces(n-len(r)) + s
	}
	return trunc(s, n)
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// itoa formats a non-negative-friendly int without pulling in strconv at call
// sites that just want a label.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// splitLines splits on newlines, trimming \r and dropping blank lines.
func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, "\r"))
		}
	}
	return out
}

// fmtDuration renders an elapsed time as M:SS or H:MM:SS, like a stopwatch.
func fmtDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
