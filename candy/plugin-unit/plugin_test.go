package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/spec"
)

// fakeExec answers several systemctl subcommands: the unit verb runs `cat` and then,
// when the step declares them, `is-enabled` and `is-active`.
type reply struct {
	out  string
	exit int
}

type fakeExec struct {
	replies map[string]reply
	seen    []string
}

func (f *fakeExec) RunCapture(_ context.Context, cmd string) (string, string, int, error) {
	f.seen = append(f.seen, cmd)
	for sub, r := range f.replies {
		if strings.Contains(cmd, sub) {
			return r.out, "", r.exit, nil
		}
	}
	return "", "no fake response for: " + cmd, 127, nil
}
func (f *fakeExec) Kind() string { return "container" }

func (f *fakeExec) ran(sub string) bool {
	for _, c := range f.seen {
		if strings.Contains(c, sub) {
			return true
		}
	}
	return false
}

// fakeCC is a fake kit.CheckContext exercising the user verb's Exec leg.
type fakeCC struct{ exec kit.Executor }

func (c *fakeCC) Exec() kit.Executor { return c.exec }
func (c *fakeCC) Mode() kit.RunMode  { return kit.ModeLive }
func (c *fakeCC) HTTPDo(context.Context, kit.HTTPRequest) (kit.HTTPResponse, error) {
	return kit.HTTPResponse{}, nil
}
func (c *fakeCC) ResolveEndpoint(context.Context, int) (string, error) { return "", nil }
func (c *fakeCC) ResolveGraphicsEndpoint(context.Context, string) (kit.GraphicsEndpoint, error) {
	return kit.GraphicsEndpoint{}, nil
}
func (c *fakeCC) ResolveImageLabel(context.Context, string) (string, error) { return "", nil }
func (c *fakeCC) DialTimeout() time.Duration                                { return 3 * time.Second }
func (c *fakeCC) Box() string                                               { return "" }
func (c *fakeCC) Instance() string                                          { return "" }
func (c *fakeCC) Distros() []string                                         { return nil }
func (c *fakeCC) AddBackground(int)                                         {}

func cc(replies map[string]reply) (*fakeCC, *fakeExec) {
	e := &fakeExec{replies: replies}
	return &fakeCC{exec: e}, e
}

func run(t *testing.T, in map[string]any, replies map[string]reply) (kit.Result, *fakeExec) {
	t.Helper()
	c, e := cc(replies)
	return verb{}.RunVerb(context.Background(), c, &spec.Op{PluginInput: in}), e
}

// Existence is probed with `systemctl cat`, not a file test. A unit written to a
// directory this manager does not read exists on disk and is still not a unit — the
// failure a plain [ -e ] cannot see.
func TestUnitVerb_ExistenceUsesSystemctlCat(t *testing.T) {
	res, e := run(t, map[string]any{"unit": "cstream-broker.socket"},
		map[string]reply{"systemctl cat": {"# /etc/systemd/system/cstream-broker.socket\n", 0}})
	if res.Status != kit.StatusPass {
		t.Errorf("expected pass, got %+v", res)
	}
	if !e.ran("systemctl cat") {
		t.Error("existence was not probed via systemctl cat")
	}

	res, _ = run(t, map[string]any{"unit": "missing.socket"},
		map[string]reply{"systemctl cat": {"", 1}})
	if res.Status != kit.StatusFail {
		t.Errorf("an unknown unit passed: %+v", res)
	}
	if !strings.Contains(res.Message, "does not read") {
		t.Errorf("the failure does not name the likely cause: %q", res.Message)
	}
}

// enable/active are TRI-STATE: unset must not probe at all. A step that only asserts
// existence on a socket started by another unit's Requires= must not be forced to care
// whether it is enabled.
func TestUnitVerb_UnsetFieldsAreNotProbed(t *testing.T) {
	_, e := run(t, map[string]any{"unit": "a.socket"},
		map[string]reply{"systemctl cat": {"x", 0}})
	if e.ran("is-enabled") || e.ran("is-active") {
		t.Errorf("probed a field the step never declared: %v", e.seen)
	}
}

func TestUnitVerb_EnabledAndActive(t *testing.T) {
	for _, tc := range []struct {
		name   string
		in     map[string]any
		rep    map[string]reply
		status kit.Status
	}{
		{"enabled and wanted", map[string]any{"unit": "a.socket", "enable": true},
			map[string]reply{"systemctl cat": {"x", 0}, "is-enabled": {"enabled", 0}}, kit.StatusPass},
		{"disabled but wanted", map[string]any{"unit": "a.socket", "enable": true},
			map[string]reply{"systemctl cat": {"x", 0}, "is-enabled": {"disabled", 1}}, kit.StatusFail},
		{"active and wanted", map[string]any{"unit": "a.socket", "active": true},
			map[string]reply{"systemctl cat": {"x", 0}, "is-active": {"active", 0}}, kit.StatusPass},
		{"inactive but wanted", map[string]any{"unit": "a.socket", "active": true},
			map[string]reply{"systemctl cat": {"x", 0}, "is-active": {"inactive", 3}}, kit.StatusFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, _ := run(t, tc.in, tc.rep)
			if res.Status != tc.status {
				t.Errorf("status = %v, want %v (%s)", res.Status, tc.status, res.Message)
			}
		})
	}
}

// A drop-in has no identity of its own: `systemctl is-enabled 50-x.conf` is meaningless.
// The probe targets the unit the fragment extends, and checks the fragment is actually
// among the resolved fragments — a file that exists but is not applied is the failure
// this catches.
func TestUnitVerb_DropInProbesItsTarget(t *testing.T) {
	res, e := run(t,
		map[string]any{"unit": "50-cstream.conf", "drop_in_for": "user-.slice"},
		map[string]reply{"systemctl cat": {
			"# /usr/lib/systemd/system/user-.slice\n# /etc/systemd/system/user-.slice.d/50-cstream.conf\n", 0}})
	if res.Status != kit.StatusPass {
		t.Errorf("expected pass, got %+v", res)
	}
	if !e.ran("user-.slice") || e.ran("cat '50-cstream.conf'") {
		t.Errorf("the probe did not target the drop-in's unit: %v", e.seen)
	}

	// Present on disk but not among the resolved fragments.
	res, _ = run(t,
		map[string]any{"unit": "50-cstream.conf", "drop_in_for": "user-.slice"},
		map[string]reply{"systemctl cat": {"# /usr/lib/systemd/system/user-.slice\n", 0}})
	if res.Status != kit.StatusFail {
		t.Errorf("an unapplied drop-in passed: %+v", res)
	}
	if !strings.Contains(res.Message, "not applied") {
		t.Errorf("the failure does not say the fragment is unapplied: %q", res.Message)
	}
}

// The act is the point of the verb: write, reload and enable in ONE step. Hand-rolled
// installs split the write from the reload, so a failure between them leaves a unit on
// disk the manager has never read.
func TestUnitVerb_ActInstallsInOneStep(t *testing.T) {
	script, ok := verb{}.RenderProvisionScript(&spec.Op{PluginInput: map[string]any{
		"unit": "cstream-broker.socket", "enable": true,
		"content": "[Unit]\nDescription=x\n[Socket]\nListenStream=/run/cstream/broker.sock\n",
	}}, nil)
	if !ok {
		t.Fatal("act declined a step that declares content")
	}
	for _, want := range []string{
		"mkdir -p '/etc/systemd/system'",
		"cat > '/etc/systemd/system/cstream-broker.socket'",
		"systemctl daemon-reload",
		"systemctl enable 'cstream-broker.socket'",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("act is missing %q:\n%s", want, script)
		}
	}
	// The heredoc delimiter must be QUOTED: unit files are full of $ and %, and %i /
	// $MAINPID belong to systemd, not the shell.
	if !strings.Contains(script, "<<'CHARLY_UNIT_EOF'") {
		t.Errorf("heredoc is unquoted; the shell will expand systemd specifiers:\n%s", script)
	}
	// Order is load-bearing: reload must follow the write, and enable must follow reload.
	w, r, e := strings.Index(script, "cat >"), strings.Index(script, "daemon-reload"), strings.Index(script, "enable")
	if w >= r || r >= e {
		t.Errorf("write/reload/enable are out of order:\n%s", script)
	}
}

// A probe-only step has nothing to provision. Rendering an empty script would make
// charly believe it acted.
func TestUnitVerb_ActDeclinesWithoutContent(t *testing.T) {
	_, ok := verb{}.RenderProvisionScript(&spec.Op{
		PluginInput: map[string]any{"unit": "a.socket", "enable": true}}, nil)
	if ok {
		t.Error("act accepted a step with no content")
	}
}

// Scope is a FIELD, never a branch on an init name — the reason this verb is `unit` and
// not `systemd_unit`.
func TestUnitVerb_UserScope(t *testing.T) {
	script, _ := verb{}.RenderProvisionScript(&spec.Op{PluginInput: map[string]any{
		"unit": "cstream-session.target", "scope": "user", "enable": true, "content": "[Unit]\n",
	}}, nil)
	if !strings.Contains(script, "systemctl --user daemon-reload") {
		t.Errorf("user scope did not reach systemctl:\n%s", script)
	}
	if !strings.Contains(script, "${HOME}/.config/systemd/user") {
		t.Errorf("user scope wrote to the system directory:\n%s", script)
	}
}

// A drop-in lands under its target's .d directory, not beside it.
func TestUnitVerb_ActDropInPath(t *testing.T) {
	script, _ := verb{}.RenderProvisionScript(&spec.Op{PluginInput: map[string]any{
		"unit": "50-cstream.conf", "drop_in_for": "user-.slice", "content": "[Slice]\n",
	}}, nil)
	if !strings.Contains(script, "'/etc/systemd/system/user-.slice.d'") {
		t.Errorf("drop-in did not land under the target's .d dir:\n%s", script)
	}
}
