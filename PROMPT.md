# Project prompt: Omarchy shell plugin for Mouse Without Borders (`mwb`)

Give this whole file to a fresh Claude Code session started in this directory
(`~/path/to/omarchy-mwb-plugin`). Load the `omarchy`
skill first and read its `plugins.md` guide before writing any code.

## Goal

An Omarchy shell **bar-widget plugin** (one bar icon + one `KeyboardPanel`
panel) that lets me manage the `mwb` Linux client without hand-editing TOML:

1. **Edit settings**: Windows host IP/hostname and the security key (plus a few
   secondary options, see below).
2. **Manage the service**: show status and Start / Stop / Restart it.
3. Show live connection state at a glance (icon colour/tooltip).

Single user, single machine, no network features beyond what `mwb` already does.

## What `mwb` is (the thing being controlled)

- A Go client for PowerToys Mouse Without Borders, installed as a **systemd
  user service**: unit `mwb.service`, binary `~/go/bin/mwb`.
  - Control with `systemctl --user {start|stop|restart|is-active|status} mwb`.
  - Logs: `journalctl --user -u mwb`.
- Config file: **`~/.config/mwb/config.toml`**, mode **0600**, dir 0700. The
  service reads it **only at startup**, so a settings change needs a restart.
- Config keys (TOML):

  | key | type | notes |
  |---|---|---|
  | `host` | string | Windows host IP or hostname (what I change most; it moves between LAN `192.168.x.x` and Tailscale `100.x.x.x`) |
  | `key` | string | MWB security key. **Secret.** |
  | `name` | string | this machine's name |
  | `port` | int | 0 = default 15100 base |
  | `clipboard` | bool | clipboard sharing |
  | `file_fetch` | string | `immediate` \| `on_switch` \| `off` (when to download files copied on Windows) |
  | `legacy_crypto` | bool \| unset | true only for PowerToys <= 0.100.2 |
  | `listen_addr` | string | optional IP to bind the listeners to |
  | `edge`, `edges`, `bidirectional`, `switch_modifier`, `keyboard_layout`, `accel_multiplier`, `inbound_multiplier` | misc | leave untouched unless exposed deliberately |

- Existing alternative: `mwb gui` serves a local web UI (loopback, per-process
  token, CSRF-guarded). **Do not use or depend on it**; the plugin edits the
  TOML file directly.

## Requirements

### Settings editing
- Fields in the panel: Host, Key (password-style, masked, with a show/hide
  toggle), `file_fetch` (3-way select), clipboard (switch). Others optional.
- **Preserve every key the panel does not show.** Read-modify-write the TOML;
  never rewrite the file from only the fields in the form (that bug already bit
  the project once). Prefer an in-place, line-oriented update of just the
  changed keys, or a real TOML parser; do not hand-roll something that drops
  comments/unknown keys.
- Write atomically (temp file in the same dir + rename) and keep **0600**.
  Never create the file world/group readable, even briefly (set the umask or
  `chmod` before writing content).
- Validate before saving: host must be a valid IP or hostname; `file_fetch` one
  of the three values; key non-empty.
- A **"Save & restart"** action that saves then runs
  `systemctl --user restart mwb`, plus a plain "Save" that doesn't restart.
- Show a clear "changes need a restart" hint when the saved config differs from
  what the running service started with.

### Service control
- Status poll (e.g. every 3-5 s while the panel is open, slower or off when
  closed): `systemctl --user is-active mwb` -> active / inactive / failed.
- Buttons: Start, Stop, Restart. Disable the ones that make no sense for the
  current state. Show errors from `systemctl` in the panel (plain text).
- Optional but nice: derive "connected to <remote name>" by reading the last
  relevant `journalctl --user -u mwb` lines (`handshake complete`,
  `connected (outbound)`, repeated `connecting` = not connected). Bound the
  lines read (e.g. `-n 50`) and never display raw log text unescaped.
- Optional: a "Test host" button that checks the host is reachable on TCP
  15101 (short timeout) before saving.

### Security (this plugin handles a secret)
- The key must **never** appear in: command lines (visible in `ps`), logs,
  notifications, tooltips, the shell's own config (`shell.json`), or any file
  other than `config.toml`. Pass values to helper commands via stdin or a
  0600 temp file, never as argv.
- Mask the key in the UI by default; don't pre-fill it into any other
  component's state longer than needed; clear it from memory/state when the
  panel closes.
- Follow `plugins.md` "Plugin UI Safety": every `Text` that shows dynamic data
  (host, errors, log lines, command output) must set `textFormat:
  Text.PlainText`. Never interpolate untrusted values into shell commands;
  use argument arrays, not `sh -c` strings.
- No network access from the plugin except the explicit optional TCP
  reachability test.
- Do not edit anything under `/usr/share/omarchy/`; the plugin lives in a user
  plugin dir only.

### Omarchy conventions (from the `omarchy` skill / `plugins.md`)
- Plugin id of the form `<author>.mwb` (confirm the
  author slug with me). Develop in `~/.config/omarchy/plugins/<id>/`
  (hot-reloads on save; force with `omarchy-shell shell rescanPlugins`), then
  keep the source of truth in **this project directory** and symlink or copy
  it into the plugins dir.
- Files, modelled on the built-in `agents` plugin
  (`/usr/share/omarchy/shell/plugins/agents/`: read `manifest.json`,
  `Main.qml`, `Panel.qml`, `README.md` there as the reference):
  - `manifest.json` with `schemaVersion: 1`, `kinds: ["bar-widget"]`,
    `entryPoints.barWidget: "Panel.qml"`, `barWidget` metadata, and a
    `schema` array for any bar-level options.
  - `Panel.qml` (bar icon + `KeyboardPanel` built on `qs.Ui`), with logic in
    separate QML/JS files where it keeps things readable.
  - `i18n/en.json` read via `Commons.Translations` (English only is fine;
    use `strings.t()` inside bindings, `%1` placeholders, no string gluing).
  - `README.md` with install/usage, the security notes above, and a
    screenshot if easy.
- Motion: use `Style.duration(ms)`, ease-out only, under 300 ms, no scale from
  0 (see `plugins.md` "Plugin Motion"). Prefer the kit's `Button`, `PopupCard`,
  `KeyboardPanel`; don't animate things that open/close many times a day.
- Add the widget to the bar with `omarchy bar` (or `shell.json`), only after it
  works; confirm the layout section with me first.

### Process / quality
- Test-first for the logic that matters (TOML read-modify-write preserves
  unknown keys and comments; atomic write keeps 0600; validation rules;
  status parsing; key never in argv). Put pure logic in testable
  JS/Python/Go helpers rather than inside QML.
- Test against a **copy** of the config (`MWB_CONFIG`-style override or a temp
  path), never my live `~/.config/mwb/config.toml`, until I say so. Do not
  restart or stop my running `mwb` service during development without asking;
  it is currently connected and in use.
- Verify visually: open the panel and screenshot it
  (`omarchy capture screenshot`) before claiming it works.
- Commit in small steps with co-authors **DeepSeek** and **Claude**. **Do not
  push** anywhere.

## Deliverables
1. The plugin directory (manifest, QML, helpers, i18n, README).
2. Tests plus the command to run them.
3. A short `INSTALL.md`: how to link it into `~/.config/omarchy/plugins/`, add
   it to the bar, and remove it.
4. A brief security note listing how the key is handled.

## Open questions to settle with me before building
- Plugin id / author slug (`<author>.mwb`?).
- Which bar section the icon goes in (`right` next to the tray?).
- Helper language for the config read/write: Python (`tomllib` is read-only,
  so a writer is needed), a tiny Go helper reusing the repo's
  `internal/config`, or pure QML/JS. My leaning: **a small Go helper** built
  from the existing config package, because it already preserves/validates
  fields and handles 0600 atomic saves. Check that before inventing new code.
- Whether the plugin should also expose the connection-state icon colours
  (green connected / amber connecting / red stopped).

## Useful paths
- Repo (mwb source, config package): `~/path/to/WaylandWithoutBorders`
  (`internal/config/config.go`, `cmd/mwb/gui.go`, `mwb.service`)
- Reference plugin: `/usr/share/omarchy/shell/plugins/agents/` (read only)
- Other user plugins for style: `~/.config/omarchy/plugins/`
- Omarchy skill guide: `~/.claude/skills/omarchy/plugins.md`
