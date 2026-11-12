package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Conn string

const (
	ConnConnected  Conn = "connected"
	ConnConnecting Conn = "connecting"
	ConnUnknown    Conn = "unknown"
)

// ParseConn reads journal lines (oldest first) and reports the latest state.
func ParseConn(lines string) Conn {
	state := ConnUnknown
	for _, l := range strings.Split(lines, "\n") {
		switch {
		case strings.Contains(l, "handshake complete"), strings.Contains(l, "connected (outbound)"):
			state = ConnConnected
		case strings.Contains(l, "connecting"):
			state = ConnConnecting
		}
	}
	return state
}

type Show struct {
	State string
	Since int64
}

func ParseShow(out string) Show {
	s := Show{State: "unknown"}
	for _, l := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		switch k {
		case "ActiveState":
			switch v {
			case "active", "inactive", "failed", "activating", "deactivating":
				s.State = v
			}
		case "ActiveEnterTimestamp":
			if n, err := strconv.ParseInt(strings.TrimPrefix(v, "@"), 10, 64); err == nil {
				s.Since = n
			}
		}
	}
	return s
}

// NeedsRestart: the running service read its config at startup, so a file
// written after that moment has not been applied yet.
func NeedsRestart(state string, serviceSince, configMtime int64) bool {
	return state == "active" && serviceSince > 0 && configMtime > serviceSince
}

// SanitizeText makes command output safe to show: no control characters,
// bounded length.
func SanitizeText(s string, max int) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(' ')
		case r < 0x20 || r == 0x7f:
		default:
			b.WriteRune(r)
		}
		if b.Len() >= max {
			break
		}
	}
	out := strings.TrimSpace(b.String())
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func validAction(a string) bool { return a == "start" || a == "stop" || a == "restart" }

func fmtView(v View) string { return fmt.Sprintf("%+v", v) }
