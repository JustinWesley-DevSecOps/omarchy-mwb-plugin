package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `# my mwb config
host = "100.64.0.10"   # tailscale
key = "SECRETKEY12345"
name = "Omarchy-x1y2"
port = 0
clipboard = true
edges = ["left", "right"]
file_fetch = "on_switch"

[extra]
host = "do-not-touch"
`

func writeTmp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func ptr[T any](v T) *T { return &v }

func TestUpdatePreservesUnknownKeysAndComments(t *testing.T) {
	p := writeTmp(t, sample)
	err := Update(p, Changes{Host: ptr("192.168.1.5"), FileFetch: ptr("off"), Clipboard: ptr(false)})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	got := string(b)
	for _, want := range []string{
		"# my mwb config", `name = "Omarchy-x1y2"`, `edges = ["left", "right"]`, "port = 0",
		`key = "SECRETKEY12345"`, "[extra]", `host = "do-not-touch"`,
		`host = "192.168.1.5"   # tailscale`, `file_fetch = "off"`, "clipboard = false",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "100.64.0.10") {
		t.Error("old host still present")
	}
}

func TestUpdateKeyReplacedOnlyWhenGiven(t *testing.T) {
	p := writeTmp(t, sample)
	if err := Update(p, Changes{Host: ptr("h.example")}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "SECRETKEY12345") {
		t.Error("key lost when not supplied")
	}
	if err := Update(p, Changes{Key: ptr("NEWKEY")}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(p)
	if !strings.Contains(string(b), `key = "NEWKEY"`) || strings.Contains(string(b), "SECRETKEY12345") {
		t.Errorf("key not replaced:\n%s", b)
	}
}

func TestUpdateAddsMissingKeyBeforeFirstTable(t *testing.T) {
	p := writeTmp(t, "host = \"a.b\"\nkey = \"k\"\n\n[extra]\nx = 1\n")
	if err := Update(p, Changes{FileFetch: ptr("immediate")}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	i := strings.Index(s, `file_fetch = "immediate"`)
	j := strings.Index(s, "[extra]")
	if i < 0 || j < 0 || i > j {
		t.Errorf("key not inserted before table:\n%s", s)
	}
}

func TestUpdateDoesNotTouchKeysInsideTables(t *testing.T) {
	p := writeTmp(t, "host = \"a.b\"\nkey = \"k\"\n[t]\nclipboard = true\n")
	if err := Update(p, Changes{Clipboard: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "[t]\nclipboard = true") {
		t.Errorf("table key modified:\n%s", b)
	}
}

func TestUpdateKeepsMode0600AndNoTempLeft(t *testing.T) {
	p := writeTmp(t, sample)
	os.Chmod(p, 0o644) // simulate a bad pre-existing mode
	if err := Update(p, Changes{Host: ptr("x.example")}); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", st.Mode().Perm())
	}
	ents, _ := os.ReadDir(filepath.Dir(p))
	if len(ents) != 1 {
		t.Errorf("temp files left behind: %v", ents)
	}
}

func TestUpdateTempFileNeverWiderThan0600(t *testing.T) {
	// The temp file must be created 0600 from the start.
	old := tempMode
	defer func() { tempMode = old }()
	var seen os.FileMode
	tempMode = func(m os.FileMode) { seen = m }
	p := writeTmp(t, sample)
	if err := Update(p, Changes{Host: ptr("x.example")}); err != nil {
		t.Fatal(err)
	}
	if seen != 0o600 {
		t.Errorf("temp created with %v", seen)
	}
}

func TestValidate(t *testing.T) {
	bad := []Changes{
		{Host: ptr("")}, {Host: ptr("bad host")}, {Host: ptr("a_b.example")}, {Host: ptr("-x.example")},
		{Host: ptr("1.2.3.999")}, {Host: ptr("host\nkey=1")}, {Host: ptr(`a"b`)},
		{FileFetch: ptr("sometimes")}, {Key: ptr("")}, {Key: ptr("a\"b")}, {Key: ptr("a\nb")},
		{Key: ptr(`a\b`)},
	}
	for _, c := range bad {
		if err := c.Validate(); err == nil {
			t.Errorf("expected error for %+v", c)
		}
	}
	good := []Changes{
		{Host: ptr("192.168.1.2")}, {Host: ptr("100.64.0.10")}, {Host: ptr("my-pc.local")},
		{Host: ptr("fe80::1")}, {Host: ptr("pc")}, {FileFetch: ptr("on_switch")},
		{FileFetch: ptr("immediate")}, {FileFetch: ptr("off")}, {Key: ptr("abcd1234")},
	}
	for _, c := range good {
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error for %+v: %v", c, err)
		}
	}
}

func TestUpdateRejectsInvalidWithoutWriting(t *testing.T) {
	p := writeTmp(t, sample)
	if err := Update(p, Changes{Host: ptr("bad host")}); err == nil {
		t.Fatal("expected error")
	}
	b, _ := os.ReadFile(p)
	if string(b) != sample {
		t.Error("file modified despite validation error")
	}
}

func TestReadNeverReturnsKey(t *testing.T) {
	p := writeTmp(t, sample)
	v, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if v.Host != "100.64.0.10" || !v.KeySet || !v.Clipboard || v.FileFetch != "on_switch" {
		t.Errorf("%+v", v)
	}
	if strings.Contains(fmtView(v), "SECRETKEY") {
		t.Error("key leaked in view")
	}
}

func TestReadDefaults(t *testing.T) {
	p := writeTmp(t, "host = \"h\"\n")
	v, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if v.KeySet || !v.Clipboard || v.FileFetch != "immediate" {
		t.Errorf("%+v", v)
	}
}

func TestParseStatus(t *testing.T) {
	cases := []struct {
		name  string
		lines string
		want  Conn
	}{
		{"connected", "connecting to x\nhandshake complete\nconnected (outbound)\n", ConnConnected},
		{"connecting", "connected (outbound)\nconnecting to 1.2.3.4\nconnecting to 1.2.3.4\n", ConnConnecting},
		{"unknown", "something else\n", ConnUnknown},
		{"empty", "", ConnUnknown},
	}
	for _, c := range cases {
		if got := ParseConn(c.lines); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestParseServiceShow(t *testing.T) {
	st := ParseShow("ActiveState=active\nActiveEnterTimestamp=@1794468168\n")
	if st.State != "active" || st.Since != 1794468168 {
		t.Errorf("%+v", st)
	}
	st = ParseShow("ActiveState=inactive\nActiveEnterTimestamp=\n")
	if st.State != "inactive" || st.Since != 0 {
		t.Errorf("%+v", st)
	}
	st = ParseShow("ActiveState=weird\n")
	if st.State != "unknown" {
		t.Errorf("%+v", st)
	}
}

func TestNeedsRestart(t *testing.T) {
	if !NeedsRestart("active", 100, 200) {
		t.Error("config newer than start should need restart")
	}
	if NeedsRestart("active", 300, 200) {
		t.Error("config older than start")
	}
	if NeedsRestart("inactive", 100, 200) {
		t.Error("inactive service never needs restart")
	}
}

func TestSanitizeError(t *testing.T) {
	s := SanitizeText("fail\x1b[31m\x00 bad\nline", 100)
	if strings.ContainsAny(s, "\x1b\x00") {
		t.Errorf("control chars: %q", s)
	}
	if len(SanitizeText(strings.Repeat("a", 500), 50)) > 50 {
		t.Error("not bounded")
	}
}

func TestActionAllowList(t *testing.T) {
	for _, a := range []string{"start", "stop", "restart"} {
		if !validAction(a) {
			t.Errorf("%s should be valid", a)
		}
	}
	for _, a := range []string{"", "enable", "kill", "restart; rm", "--user"} {
		if validAction(a) {
			t.Errorf("%s should be invalid", a)
		}
	}
}

// ---- edges and switch_modifier ----

func edgesOf(t *testing.T, content string) View {
	t.Helper()
	v, err := Read(writeTmp(t, content))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestReadEdges(t *testing.T) {
	cases := []struct {
		name, toml string
		want       []string
		editable   bool
	}{
		{"unset means all four", "host = \"h\"\n", []string{"left", "right", "top", "bottom"}, true},
		{"subset in canonical order", "edges = [\"bottom\", \"left\"]\n", []string{"left", "bottom"}, true},
		{"none", "edges = [\"none\"]\n", []string{}, true},
		{"off is none", "edges = [\"off\"]  # no edges\n", []string{}, true},
		{"empty array is all four", "edges = []\n", []string{"left", "right", "top", "bottom"}, true},
		{"unknown names ignored", "edges = [\"left\", \"diagonal\"]\n", []string{"left"}, true},
		{"multi-line array is not editable", "edges = [\n  \"left\",\n  \"right\",\n]\n", []string{"left", "right", "top", "bottom"}, false},
	}
	for _, c := range cases {
		v := edgesOf(t, c.toml)
		if strings.Join(v.Edges, ",") != strings.Join(c.want, ",") || v.EdgesEditable != c.editable {
			t.Errorf("%s: got %v editable=%v, want %v editable=%v", c.name, v.Edges, v.EdgesEditable, c.want, c.editable)
		}
		if v.Edges == nil {
			t.Errorf("%s: Edges must be a non-nil slice so it serialises as []", c.name)
		}
	}
}

func TestReadSwitchModifier(t *testing.T) {
	if v := edgesOf(t, "host = \"h\"\n"); v.SwitchModifier != "" {
		t.Errorf("unset: %q", v.SwitchModifier)
	}
	if v := edgesOf(t, "switch_modifier = \"shift\"  # easy mouse\n"); v.SwitchModifier != "shift" {
		t.Errorf("shift: %q", v.SwitchModifier)
	}
}

func TestUpdateEdgesWritesCanonicalList(t *testing.T) {
	p := writeTmp(t, sample)
	if err := Update(p, Changes{Edges: []string{"top", "left"}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	got := string(b)
	if !strings.Contains(got, `edges = ["left", "top"]`) {
		t.Errorf("edges not rewritten:\n%s", got)
	}
	for _, want := range []string{"# my mwb config", `key = "SECRETKEY12345"`, "[extra]", `file_fetch = "on_switch"`} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q", want)
		}
	}
}

func TestUpdateNoEdgesSelectedWritesNone(t *testing.T) {
	p := writeTmp(t, sample)
	if err := Update(p, Changes{Edges: []string{}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), `edges = ["none"]`) {
		t.Errorf("expected none:\n%s", b)
	}
}

func TestUpdateEdgesAddedAboveTables(t *testing.T) {
	p := writeTmp(t, "host = \"h\"\n\n[extra]\nx = 1\n")
	if err := Update(p, Changes{Edges: []string{"right"}, SwitchModifier: ptr("alt")}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	got := string(b)
	if strings.Index(got, "edges") > strings.Index(got, "[extra]") || strings.Index(got, "switch_modifier") > strings.Index(got, "[extra]") {
		t.Errorf("new keys landed inside a table:\n%s", got)
	}
	if !strings.Contains(got, `switch_modifier = "alt"`) {
		t.Errorf("modifier missing:\n%s", got)
	}
}

func TestUpdateEdgesUntouchedWhenOmitted(t *testing.T) {
	p := writeTmp(t, sample)
	if err := Update(p, Changes{Host: ptr("h.example")}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "switch_modifier") || !strings.Contains(string(b), `edges = ["left", "right"]`) {
		t.Errorf("edge settings changed without being asked:\n%s", b)
	}
}

func TestEdgeValidation(t *testing.T) {
	bad := []Changes{
		{Edges: []string{"diagonal"}},
		{Edges: []string{"left", "left"}},
		{Edges: []string{"none"}},
		{SwitchModifier: ptr("super")},
		{SwitchModifier: ptr("shift\"")},
	}
	for _, c := range bad {
		p := writeTmp(t, sample)
		if err := Update(p, c); err == nil {
			t.Errorf("accepted %+v", c)
		}
		if b, _ := os.ReadFile(p); string(b) != sample {
			t.Errorf("file modified despite rejected %+v", c)
		}
	}
	for _, m := range []string{"", "shift", "ctrl", "alt"} {
		if err := Update(writeTmp(t, sample), Changes{SwitchModifier: ptr(m)}); err != nil {
			t.Errorf("modifier %q rejected: %v", m, err)
		}
	}
}

func TestUpdateRefusesToRewriteMultiLineEdges(t *testing.T) {
	src := "edges = [\n  \"left\",\n]\nhost = \"h\"\n"
	p := writeTmp(t, src)
	if err := Update(p, Changes{Edges: []string{"right"}}); err == nil {
		t.Fatal("expected an error for a multi-line edges array")
	}
	if b, _ := os.ReadFile(p); string(b) != src {
		t.Error("file modified")
	}
}

// ---- bidirectional ----

func TestReadBidirectional(t *testing.T) {
	if v := edgesOf(t, "host = \"h\"\n"); v.Bidirectional {
		t.Error("unset must read as off")
	}
	if v := edgesOf(t, "bidirectional = true  # both ways\n"); !v.Bidirectional {
		t.Error("true not read")
	}
	if v := edgesOf(t, "bidirectional = false\n"); v.Bidirectional {
		t.Error("false read as true")
	}
}

func TestUpdateBidirectional(t *testing.T) {
	p := writeTmp(t, sample+"")
	if err := Update(p, Changes{Bidirectional: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	got := string(b)
	if !strings.Contains(got, "bidirectional = true") {
		t.Errorf("not written:\n%s", got)
	}
	if strings.Index(got, "bidirectional") > strings.Index(got, "[extra]") {
		t.Errorf("landed inside a table:\n%s", got)
	}
	if err := Update(p, Changes{Bidirectional: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(p)
	if !strings.Contains(string(b), "bidirectional = false") || strings.Count(string(b), "bidirectional") != 1 {
		t.Errorf("not replaced in place:\n%s", b)
	}
	if !strings.Contains(string(b), `key = "SECRETKEY12345"`) {
		t.Error("lost the key")
	}
}

func TestUpdateBidirectionalUntouchedWhenOmitted(t *testing.T) {
	p := writeTmp(t, sample)
	if err := Update(p, Changes{Host: ptr("h.example")}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "bidirectional") {
		t.Errorf("added without being asked:\n%s", b)
	}
}
