// The `unit` verb's OWN CUE schema — the typed plugin_input for a systemd unit FILE:
// an assert that probes it via systemctl, and an act that installs it (write →
// daemon-reload → enable). Single source, used two ways, exactly as #UserInput is:
// `cue exp gengotypes` emits ../params/cue_types_gen.go for the typed decode, and the
// plugin serves this source over Describe so the host validates every authored `unit:`
// step's plugin_input against #UnitInput.
//
// WHY THIS VERB EXISTS. charly can already declare a SERVICE (`service:`), and that is
// deliberately init-agnostic — it renders to supervisord, systemd or OpenRC. But a
// `.socket`, `.target` or `.slice` has NO cross-init analogue, so it cannot become a
// `service:` entry without giving the other two inits something to render that they have
// no concept of. Candies therefore hand-roll them today as `write:` + `command:
// systemctl daemon-reload && systemctl enable`, which is the exact "manual systemctl
// against a charly-managed resource" R4 forbids: the write is unowned, the reload is
// unordered, and nothing can ASSERT the result.
//
// WHY IT IS CALLED `unit` AND NOT `systemd_unit`. A reserved word naming a concrete init
// re-creates what the data-driven `init:` kind eliminated. The init is a FIELD of the
// input, not part of the verb's name.
//
// NOT `.mount`. plugin-mount already owns that with #MountInput{mount, mount_source,
// filesystem, opt}; a mount authored here would be a second way to say the same thing.
// Reuse the seam that exists.
#UnitInput: {
	// unit — the unit FILE name, including its extension: "cstream-broker.socket",
	// "cstream-session.target", "50-cstream.conf" for a drop-in. The verb
	// discriminator, and the name systemctl is asked about.
	unit: string & !="" @go(Unit)

	// scope — whose manager owns it. "system" writes to /etc/systemd/system and drives
	// `systemctl`; "user" writes to ~/.config/systemd/user and drives `systemctl --user`.
	// Defaults to system.
	scope?: "system" | "user"

	// content — the unit text. Setting it makes the step an INSTALL (act); leaving it
	// unset makes the step a pure probe of a unit something else installed.
	content?: string & !="" @go(Content)

	// drop_in_for — install `unit` as a DROP-IN of the named unit, under
	// <drop_in_for>.d/<unit>, rather than as a unit of its own. This is how a
	// `user-.slice.d/50-cstream.conf` is authored: the file is a fragment, not a unit,
	// so systemctl is never asked about the fragment's own name.
	drop_in_for?: string & !="" @go(DropInFor)

	// enable — assert `systemctl is-enabled` (probe) / run `systemctl enable` after the
	// write (act). Tri-state: unset asserts nothing and enables nothing, so a unit that
	// is only ever started by another unit's Requires= is not forced into an [Install]
	// section it does not have.
	enable?: bool @go(Enable,type=*bool)

	// active — assert `systemctl is-active`. PROBE ONLY: starting a unit is a
	// deployment action, not provisioning, and `service:` already owns "this should be
	// running". Tri-state.
	active?: bool @go(Active,type=*bool)
}
