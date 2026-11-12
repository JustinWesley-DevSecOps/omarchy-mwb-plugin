# Mouse Without Borders (Omarchy shell plugin)

One bar icon and one panel for the `mwb` client: edit its settings and
start, stop or restart its systemd user service, without hand-editing
`~/.config/mwb/config.toml`.

Plugin id: `sippin45.mwb`

## Panel

- **Status** line: Running / Stopped / Failed, plus connected or connecting
  (read from the last 50 journal lines of `mwb.service`).
- **On/off switch** beside the title: on while the service runs. Flipping it runs `systemctl --user start|stop mwb`; `systemctl` errors show as text. To apply saved settings, turn it off and on.
- **Settings**: Windows host (with a *Test host* button that tries a TCP
  connect to the control port + 1, 2 s timeout), security key (masked, with
  Show/Hide), `file_fetch` (3-way select) and clipboard sharing.
- **Save** writes the file. If the saved file is newer than the running service, the panel says
  the changes are not applied yet.

The icon is a single neutral glyph; connection state shows in the panel and
the tooltip, not in the icon colour.

## How it works

QML does no file or service work itself. It calls a small Go helper,
`bin/mwbcfg` (source in `helper/`):

| command | what it does |
|---|---|
| `get` | the editable settings as JSON; the key is never included (only `keySet`) |
| `set` | reads one JSON line on stdin, updates only the given keys |
| `status` | `systemctl --user show` + bounded journal read, parsed |
| `action start\|stop\|restart` | allow-listed `systemctl --user <a> mwb` |
| `test-host <host>` | TCP reachability check |

`set` edits `config.toml` **in place, line by line**. Only `host`, `key`,
`file_fetch` and `clipboard` are touched; every other key, comment and table is
preserved byte for byte. The file is written to a temp file created `0600`,
synced, and renamed over the original, so it is never group/world readable,
even briefly.

## Security notes

- The key is never in argv (so not in `ps`), logs, notifications, tooltips,
  `shell.json`, or any file other than `config.toml`. It reaches the helper on
  stdin only.
- The panel never loads the existing key. The field is empty and the config is
  only changed if you type a new key. The key is cleared when the panel closes.
- Values are validated before writing (host is an IP or hostname, `file_fetch`
  is one of three values, key is non-empty and has no quotes, backslashes or
  control characters), and rejected rather than escaped.
- Every `Text` that shows dynamic data sets `textFormat: Text.PlainText`.
  Helper errors are stripped of control characters and length-limited.
- Commands run as argument arrays, never through `sh -c`. The `action` command
  accepts only `start`, `stop`, `restart`.
- The only network access is the explicit *Test host* TCP connect.

## Tests

```sh
./test.sh
```

Runs `gofmt`, `go vet`, the helper's Go tests (comment/unknown-key
preservation, atomic 0600 write, validation, status parsing, action
allow-list) and static checks on the QML (key not in argv, all `Text` is
plain, no shell strings, key cleared on close).

## Install

See [INSTALL.md](INSTALL.md).
