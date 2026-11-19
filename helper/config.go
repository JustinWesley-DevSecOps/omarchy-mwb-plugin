package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Changes holds the settings the panel may edit. A nil field is left alone.
type Changes struct {
	Host      *string `json:"host"`
	Key       *string `json:"key"`
	FileFetch *string `json:"file_fetch"`
	Clipboard *bool   `json:"clipboard"`
	// Edges: nil leaves the setting alone, an empty slice means no edge
	// switches (written as ["none"]).
	Edges          []string `json:"edges"`
	SwitchModifier *string  `json:"switch_modifier"`
	Bidirectional  *bool    `json:"bidirectional"`
}

// View is what the panel may see. It never contains the key.
type View struct {
	Host      string `json:"host"`
	KeySet    bool   `json:"keySet"`
	FileFetch string `json:"fileFetch"`
	Clipboard bool   `json:"clipboard"`
	Port      int    `json:"port"`
	// Edges is the resolved list of edges that switch to the remote, in
	// canonical order. Never nil, so it serialises as [] when none are enabled.
	Edges []string `json:"edges"`
	// EdgesEditable is false when the edges array spans several lines, which
	// this helper does not rewrite.
	EdgesEditable  bool   `json:"edgesEditable"`
	SwitchModifier string `json:"switchModifier"`
	Bidirectional  bool   `json:"bidirectional"`
}

var allEdges = []string{"left", "right", "top", "bottom"}

func validEdge(e string) bool {
	for _, a := range allEdges {
		if e == a {
			return true
		}
	}
	return false
}

// canonicalEdges orders a set of edges left, right, top, bottom.
func canonicalEdges(in []string) []string {
	out := []string{}
	for _, a := range allEdges {
		for _, e := range in {
			if e == a {
				out = append(out, a)
				break
			}
		}
	}
	return out
}

func multiLineArray(raw string) bool {
	return strings.HasPrefix(raw, "[") && !strings.Contains(raw, "]")
}

// resolveEdges mirrors mwb's own EnabledEdges: unset or empty means all four,
// "none"/"off" means none, otherwise the valid names listed.
func resolveEdges(raw string) []string {
	body := strings.TrimSpace(raw)
	body = strings.TrimPrefix(body, "[")
	body = strings.TrimSuffix(body, "]")
	var names []string
	for _, part := range strings.Split(body, ",") {
		if n := unquote(strings.TrimSpace(part)); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return append([]string{}, allEdges...)
	}
	var valid []string
	for _, n := range names {
		switch {
		case validEdge(n):
			valid = append(valid, n)
		case n == "none" || n == "off":
			return []string{}
		}
	}
	return canonicalEdges(valid)
}

func edgesLiteral(edges []string) string {
	e := canonicalEdges(edges)
	if len(e) == 0 {
		return `["none"]`
	}
	q := make([]string, len(e))
	for i, n := range e {
		q[i] = quote(n)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

var hostLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

func validHost(h string) bool {
	if h == "" || len(h) > 253 {
		return false
	}
	if net.ParseIP(h) != nil {
		return true
	}
	// Anything that looks like a dotted number must be a real IPv4 address.
	if regexp.MustCompile(`^[0-9.]+$`).MatchString(h) {
		return false
	}
	for _, l := range strings.Split(h, ".") {
		if !hostLabel.MatchString(l) {
			return false
		}
	}
	return true
}

// plainString: values are written as TOML basic strings without escaping, so
// anything that would need escaping is rejected instead.
func plainString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f || r == '"' || r == '\\' {
			return false
		}
	}
	return true
}

func (c Changes) Validate() error {
	if c.Host != nil && !validHost(*c.Host) {
		return errors.New("host must be an IP address or a valid hostname")
	}
	if c.Key != nil && !plainString(*c.Key) {
		return errors.New("key must not be empty or contain quotes, backslashes or control characters")
	}
	if c.FileFetch != nil {
		switch *c.FileFetch {
		case "immediate", "on_switch", "off":
		default:
			return errors.New("file_fetch must be immediate, on_switch or off")
		}
	}
	if c.Edges != nil {
		seen := map[string]bool{}
		for _, e := range c.Edges {
			if !validEdge(e) {
				return errors.New("edges must be left, right, top or bottom")
			}
			if seen[e] {
				return errors.New("edges must not repeat")
			}
			seen[e] = true
		}
	}
	if c.SwitchModifier != nil {
		switch *c.SwitchModifier {
		case "", "shift", "ctrl", "alt":
		default:
			return errors.New("switch_modifier must be empty, shift, ctrl or alt")
		}
	}
	return nil
}

// ---- line-oriented TOML access (top-level keys only) ----

var tableHeader = regexp.MustCompile(`^\s*\[\[?[A-Za-z0-9_.\- "']+\]\]?\s*(#.*)?$`)

type kv struct {
	line       int
	prefix     string // indentation + key + " = "
	value      string // raw value text
	rest       string // whitespace + trailing comment, kept as is
	stringKind bool
}

func parseLine(line, key string) (kv, bool) {
	t := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(t, key) {
		return kv{}, false
	}
	after := t[len(key):]
	a := strings.TrimLeft(after, " \t")
	if !strings.HasPrefix(a, "=") {
		return kv{}, false
	}
	indent := line[:len(line)-len(t)]
	eq := after[:len(after)-len(a)] + "="
	body := a[1:]
	v := strings.TrimLeft(body, " \t")
	gap := body[:len(body)-len(v)]
	out := kv{prefix: indent + key + eq + gap}
	if strings.HasPrefix(v, `"`) {
		i := 1
		for i < len(v) && v[i] != '"' {
			if v[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(v) {
			return kv{}, false
		}
		out.value, out.rest, out.stringKind = v[:i+1], v[i+1:], true
		return out, true
	}
	end := len(v)
	if h := strings.Index(v, "#"); h >= 0 {
		end = h
	}
	raw := v[:end]
	trimmed := strings.TrimRight(raw, " \t")
	out.value, out.rest = trimmed, raw[len(trimmed):]+v[end:]
	return out, true
}

func topLevel(lines []string, key string) (kv, bool) {
	for i, l := range lines {
		if tableHeader.MatchString(l) {
			return kv{}, false
		}
		if e, ok := parseLine(l, key); ok {
			e.line = i
			return e, true
		}
	}
	return kv{}, false
}

func firstTable(lines []string) int {
	for i, l := range lines {
		if tableHeader.MatchString(l) {
			return i
		}
	}
	return -1
}

func unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		s, err := strconv.Unquote(v)
		if err == nil {
			return s
		}
		return v[1 : len(v)-1]
	}
	return v
}

func splitLines(b []byte) []string {
	s := string(b)
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func setKey(lines []string, key, raw string) []string {
	if e, ok := topLevel(lines, key); ok {
		lines[e.line] = e.prefix + raw + e.rest
		return lines
	}
	line := key + " = " + raw
	at := firstTable(lines)
	if at < 0 {
		return append(lines, line)
	}
	// keep a blank separator line before the table where there was one
	ins := at
	for ins > 0 && strings.TrimSpace(lines[ins-1]) == "" {
		ins--
	}
	out := append([]string{}, lines[:ins]...)
	out = append(out, line)
	return append(out, lines[ins:]...)
}

func quote(s string) string { return `"` + s + `"` }

// Read returns the editable view of the config. The key is never returned.
func Read(path string) (View, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return View{}, err
	}
	lines := splitLines(b)
	v := View{Clipboard: true, FileFetch: "immediate", Edges: append([]string{}, allEdges...), EdgesEditable: true}
	if e, ok := topLevel(lines, "host"); ok {
		v.Host = unquote(e.value)
	}
	if e, ok := topLevel(lines, "key"); ok && unquote(e.value) != "" {
		v.KeySet = true
	}
	if e, ok := topLevel(lines, "clipboard"); ok && e.value == "false" {
		v.Clipboard = false
	}
	if e, ok := topLevel(lines, "file_fetch"); ok {
		if f := unquote(e.value); f != "" {
			v.FileFetch = f
		}
	}
	if e, ok := topLevel(lines, "port"); ok {
		v.Port, _ = strconv.Atoi(e.value)
	}
	if e, ok := topLevel(lines, "edges"); ok {
		if multiLineArray(e.value) {
			v.EdgesEditable = false
		} else {
			v.Edges = resolveEdges(e.value)
		}
	}
	if e, ok := topLevel(lines, "switch_modifier"); ok {
		v.SwitchModifier = unquote(e.value)
	}
	if e, ok := topLevel(lines, "bidirectional"); ok && e.value == "true" {
		v.Bidirectional = true
	}
	return v, nil
}

// tempMode is a test hook that records the mode the temp file is created with.
var tempMode = func(os.FileMode) {}

// Update edits only the given keys in place and writes the file atomically,
// owner-only from the first byte.
func Update(path string, c Changes) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := splitLines(b)
	if c.Edges != nil {
		if e, ok := topLevel(lines, "edges"); ok && multiLineArray(e.value) {
			return errors.New("edges spans several lines in config.toml; edit it by hand")
		}
	}
	if c.Host != nil {
		lines = setKey(lines, "host", quote(*c.Host))
	}
	if c.Key != nil {
		lines = setKey(lines, "key", quote(*c.Key))
	}
	if c.FileFetch != nil {
		lines = setKey(lines, "file_fetch", quote(*c.FileFetch))
	}
	if c.Clipboard != nil {
		lines = setKey(lines, "clipboard", strconv.FormatBool(*c.Clipboard))
	}
	if c.Edges != nil {
		lines = setKey(lines, "edges", edgesLiteral(c.Edges))
	}
	if c.SwitchModifier != nil {
		lines = setKey(lines, "switch_modifier", quote(*c.SwitchModifier))
	}
	if c.Bidirectional != nil {
		lines = setKey(lines, "bidirectional", strconv.FormatBool(*c.Bidirectional))
	}
	return writeAtomic(path, []byte(strings.Join(lines, "\n")+"\n"))
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".config-"+hex.EncodeToString(rnd[:])+".tmp")
	tempMode(0o600)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	fail := func(err error) error { f.Close(); os.Remove(tmp); return err }
	if err := f.Chmod(0o600); err != nil {
		return fail(err)
	}
	if _, err := f.Write(data); err != nil {
		return fail(err)
	}
	if err := f.Sync(); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
