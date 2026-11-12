// mwbcfg: helper for the Omarchy mwb plugin. Reads and edits the mwb config,
// reports and controls the user service. The key is never printed and never
// passed on a command line: `set` takes one JSON line on stdin.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func configPath() string {
	if p := os.Getenv("MWB_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "mwb", "config.toml")
}

func tool(envName, def string) string {
	if p := os.Getenv(envName); p != "" {
		return p
	}
	return def
}

func emit(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

func fail(err error) {
	emit(map[string]any{"ok": false, "error": SanitizeText(err.Error(), 300)})
}

func run(timeout time.Duration, name string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var so, se limitedBuf
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.String(), se.String(), err
}

type limitedBuf struct{ b []byte }

func (l *limitedBuf) Write(p []byte) (int, error) {
	if room := 64*1024 - len(l.b); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		l.b = append(l.b, p[:room]...)
	}
	return len(p), nil
}
func (l *limitedBuf) String() string { return string(l.b) }

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: mwbcfg get|set|status|action <a>|test-host"))
		os.Exit(2)
	}
	path := configPath()
	switch os.Args[1] {
	case "get":
		v, err := Read(path)
		if err != nil {
			fail(err)
			return
		}
		emit(map[string]any{"ok": true, "config": v})
	case "set":
		line, err := bufio.NewReaderSize(os.Stdin, 64*1024).ReadBytes('\n')
		if err != nil && len(line) == 0 {
			fail(fmt.Errorf("no input"))
			return
		}
		var c Changes
		if err := json.Unmarshal(line, &c); err != nil {
			fail(fmt.Errorf("invalid input"))
			return
		}
		if err := Update(path, c); err != nil {
			fail(err)
			return
		}
		emit(map[string]any{"ok": true})
	case "status":
		out, _, _ := run(4*time.Second, tool("MWBCFG_SYSTEMCTL", "systemctl"),
			"--user", "show", "mwb", "--timestamp=unix", "-p", "ActiveState", "-p", "ActiveEnterTimestamp")
		sh := ParseShow(out)
		var mtime int64
		if st, err := os.Stat(path); err == nil {
			mtime = st.ModTime().Unix()
		}
		conn := ConnUnknown
		if sh.State == "active" {
			j, _, _ := run(4*time.Second, tool("MWBCFG_JOURNALCTL", "journalctl"),
				"--user", "-u", "mwb", "-n", "50", "--no-pager", "-o", "cat")
			conn = ParseConn(j)
		}
		emit(map[string]any{"ok": true, "state": sh.State, "conn": conn,
			"needsRestart": NeedsRestart(sh.State, sh.Since, mtime)})
	case "action":
		if len(os.Args) < 3 || !validAction(os.Args[2]) {
			fail(fmt.Errorf("invalid action"))
			return
		}
		_, se, err := run(20*time.Second, tool("MWBCFG_SYSTEMCTL", "systemctl"), "--user", os.Args[2], "mwb")
		if err != nil {
			msg := SanitizeText(se, 300)
			if msg == "" {
				msg = err.Error()
			}
			fail(fmt.Errorf("%s", msg))
			return
		}
		emit(map[string]any{"ok": true})
	case "test-host":
		if len(os.Args) < 3 || !validHost(os.Args[2]) {
			fail(fmt.Errorf("invalid host"))
			return
		}
		port := 15100
		if v, err := Read(path); err == nil && v.Port > 0 {
			port = v.Port
		}
		addr := net.JoinHostPort(os.Args[2], strconv.Itoa(port+1))
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			emit(map[string]any{"ok": true, "reachable": false})
			return
		}
		conn.Close()
		emit(map[string]any{"ok": true, "reachable": true})
	default:
		fail(fmt.Errorf("unknown command"))
		os.Exit(2)
	}
}
