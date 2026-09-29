# plugin-unit

Systemd unit files for OpenCharly — the `unit:` verb.

The `unit:` verb owns a systemd unit **file** that has no cross-init analogue: a
`.socket`, `.target`, `.slice`, or a drop-in fragment.

`service:` is deliberately init-agnostic and renders to supervisord, systemd or
OpenRC alike. A socket or a slice has nothing to render on the other two, so
candies used to hand-roll them as `write:` + `command: systemctl daemon-reload &&
systemctl enable` — the manual `systemctl` against a charly-managed resource that
R4 forbids. This verb owns that instead: one step writes, reloads, enables, and
can be **asserted**. The init is a field, not a reserved word — hence `unit`, not
`systemd_unit`.

It is a host-coupled verb on the SDK kit contract (`CheckVerbProvider` +
`ProvisionActor`).

## What it provides

| Capability | Surface |
|---|---|
| `verb:unit` | the `unit:` typed step — probe and install a systemd unit file |

## How to use it

Compose the plugin candy in a box or check bed's `candy:` list:

```yaml
- '@github.com/opencharly/plugin-unit/candy/plugin-unit:<tag>'
```

Then author the verb in a plan:

```yaml
- run: unit=cstream-broker.socket
  unit:
    unit: cstream-broker.socket
    enable: true
    content: |
      [Unit]
      Description=cstream broker socket
      [Socket]
      ListenStream=/run/cstream/broker.sock
      [Install]
      WantedBy=sockets.target

- check: the broker socket is installed and enabled
  unit: {unit: cstream-broker.socket, enable: true}
  context: [runtime]
```

## Layout

- `candy/plugin-unit/` — the plugin module: `plugin.go` (the verb +
  `NewCheckVerb()`/`NewMeta()`), `schema/unit.cue` (the self-contained
  `#UnitInput`), `params/cue_types_gen.go`, `cmd/serve/main.go`.
- `charly.yml` — the root project manifest (`discover: candy`).
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.

## Related

- Owning skill: `/charly-core:service` — the service lifecycle surface this
  verb extends for units with no cross-init analogue. This candy carries no
  `skill:` entity of its own; the gap is tracked in
  [opencharly/opencharly#291](https://github.com/opencharly/opencharly/issues/291).
- `/charly-internals:plugin` — the plugin/provider model.
- [`opencharly/charly`](https://github.com/opencharly/charly) — the charly CLI.
