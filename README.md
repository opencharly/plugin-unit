# plugin-unit

The `unit:` check verb — a systemd unit **file** that has no cross-init analogue: a
`.socket`, `.target`, `.slice`, or a drop-in fragment.

`service:` is deliberately init-agnostic and renders to supervisord, systemd or OpenRC
alike. A socket or a slice has nothing to render on the other two, so candies hand-roll
them as `write:` + `command: systemctl daemon-reload && systemctl enable` — the manual
`systemctl` against a charly-managed resource that R4 forbids. This verb owns that
instead: one step writes, reloads, enables, and can be **asserted**.

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

Part of the OpenCharly org. See `candy/plugin-unit/schema/unit.cue` for the full input.
