// Package unit is the importable `unit` verb: a MULTI-ROLE state-provision verb for a
// systemd unit FILE that has no cross-init analogue — a .socket, .target, .slice, or a
// drop-in fragment.
//
// CHECK (kit.CheckVerbProvider): probe the unit through systemctl.
// ACT (kit.ProvisionActor): install it — write, daemon-reload, enable.
//
// It exists because `service:` is deliberately init-agnostic and renders to supervisord,
// systemd or OpenRC alike; a .socket or .slice has nothing to render on the other two.
// Candies therefore hand-roll those as `write:` + `command: systemctl daemon-reload &&
// systemctl enable`, which is the "manual systemctl against a charly-managed resource"
// R4 forbids — unowned write, unordered reload, and no way to assert the result.
package unit

import (
	"context"
	"embed"
	"fmt"
	"path"
	"strings"

	"github.com/opencharly/plugin-unit/candy/plugin-unit/params"
	"github.com/opencharly/sdk"
	"github.com/opencharly/sdk/kit"
	pb "github.com/opencharly/spec/proto"
	"github.com/opencharly/spec/shellquote"
	"github.com/opencharly/spec/spec"
)

//go:embed schema/*.cue
var schemaFS embed.FS

// NewCheckVerb returns the unit verb as a kit.CheckVerbProvider. Because verb also
// implements kit.ProvisionActor, charly registers the multi-role (check + act) adapter.
func NewCheckVerb() kit.CheckVerbProvider { return verb{} }

// NewMeta advertises verb:unit (plugin_input #UnitInput) + the embedded CUE schema.
func NewMeta() pb.PluginMetaServer {
	return sdk.NewMeta("2026.240.1900",
		[]sdk.ProvidedCapability{{Class: "verb", Word: "unit", InputDef: "#UnitInput", Primary: "unit"}},
		schemaFS)
}

type verb struct{}

func (verb) Reserved() string { return "unit" }

// systemctl returns the management command for the input's scope. The scope is a FIELD,
// never a branch on an init name — the same reason this verb is `unit` and not
// `systemd_unit`.
func systemctl(in *params.UnitInput) string {
	if in.Scope == "user" {
		return "systemctl --user"
	}
	return "systemctl"
}

// unitDir is where a unit of this scope lives. A drop-in lands under its target's .d
// directory instead, which is why the caller passes the input rather than just a scope.
func unitDir(in *params.UnitInput) string {
	base := "/etc/systemd/system"
	if in.Scope == "user" {
		// ${HOME} rather than a resolved path: the act script runs as the destination
		// user, and charly's own home token is not available inside a rendered shell.
		base = "${HOME}/.config/systemd/user"
	}
	if in.DropInFor != "" {
		return path.Join(base, in.DropInFor+".d")
	}
	return base
}

// probeName is what systemctl is asked about. A DROP-IN has no identity of its own —
// asking `systemctl is-enabled 50-cstream.conf` is meaningless — so the probe targets
// the unit the fragment extends.
func probeName(in *params.UnitInput) string {
	if in.DropInFor != "" {
		return in.DropInFor
	}
	return in.Unit
}

// RunVerb (do:assert) probes the unit through systemctl. It asserts existence always,
// and enabled/active only when the step declares them.
func (verb) RunVerb(ctx context.Context, cc kit.CheckContext, op *spec.Op) kit.Result {
	var in params.UnitInput
	kit.DecodeInput(op.PluginInput, &in)
	if in.Unit == "" {
		return kit.Fail("unit: no unit name")
	}
	sc, name := systemctl(&in), probeName(&in)

	// `systemctl cat` is the existence probe rather than a file test: it resolves the
	// unit the way the manager does, so it also catches a file written to a directory
	// this manager does not read — the failure a plain `[ -e ]` cannot see.
	out, _, exit, err := cc.Exec().RunCapture(ctx,
		fmt.Sprintf("%s cat %s", sc, shellquote.ShellQuote(name)))
	if err != nil {
		return kit.Failf("unit probe: %v", err)
	}
	if exit != 0 {
		return kit.Failf("unit %s is unknown to %s — it is not installed, or it was "+
			"written somewhere this manager does not read", name, sc)
	}
	// A drop-in must actually be part of what the manager resolved; `cat` prints every
	// fragment, so its absence here means the file exists but is not being applied.
	if in.DropInFor != "" && !strings.Contains(out, in.Unit) {
		return kit.Failf("drop-in %s is not among the fragments %s resolves for %s — "+
			"it exists but is not applied", in.Unit, sc, in.DropInFor)
	}

	if in.Enable != nil {
		_, _, ex, err := cc.Exec().RunCapture(ctx,
			fmt.Sprintf("%s is-enabled %s", sc, shellquote.ShellQuote(name)))
		if err != nil {
			return kit.Failf("is-enabled probe: %v", err)
		}
		if enabled := ex == 0; enabled != *in.Enable {
			return kit.Failf("unit %s enabled=%v, want %v", name, enabled, *in.Enable)
		}
	}
	if in.Active != nil {
		_, _, ex, err := cc.Exec().RunCapture(ctx,
			fmt.Sprintf("%s is-active %s", sc, shellquote.ShellQuote(name)))
		if err != nil {
			return kit.Failf("is-active probe: %v", err)
		}
		if active := ex == 0; active != *in.Active {
			return kit.Failf("unit %s active=%v, want %v", name, active, *in.Active)
		}
	}
	return kit.Passf("unit %s installed", name)
}

// RenderProvisionScript (do:act) installs the unit: write → daemon-reload → enable.
//
// ok is false when the step declares no content: a probe-only step has nothing to
// provision, and rendering an empty script would make `charly` believe it acted.
func (verb) RenderProvisionScript(op *spec.Op, _ []string) (string, bool) {
	var in params.UnitInput
	kit.DecodeInput(op.PluginInput, &in)
	if in.Unit == "" || in.Content == "" {
		return "", false
	}
	dir := unitDir(&in)
	dst := path.Join(dir, in.Unit)
	sc := systemctl(&in)

	var b strings.Builder
	fmt.Fprintf(&b, "mkdir -p %s\n", shellquote.ShellQuote(dir))
	// A quoted heredoc delimiter: unit files are full of $ and %, and the shell must
	// not touch either. %i, %h and $MAINPID are systemd's, not the shell's.
	fmt.Fprintf(&b, "cat > %s <<'CHARLY_UNIT_EOF'\n%s\nCHARLY_UNIT_EOF\n",
		shellquote.ShellQuote(dst), strings.TrimRight(in.Content, "\n"))
	// The reload is part of the SAME step as the write, which is the whole point:
	// hand-rolled installs put them in separate steps, where a failure between them
	// leaves a unit on disk that the manager has never read.
	fmt.Fprintf(&b, "%s daemon-reload\n", sc)
	if in.Enable != nil {
		verb := "disable"
		if *in.Enable {
			verb = "enable"
		}
		fmt.Fprintf(&b, "%s %s %s\n", sc, verb, shellquote.ShellQuote(probeName(&in)))
	}
	return strings.TrimRight(b.String(), "\n"), true
}
