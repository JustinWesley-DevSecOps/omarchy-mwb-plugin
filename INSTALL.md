# Install, enable, remove

## Install

```sh
cd ~/path/to/omarchy-mwb-plugin
./build.sh                       # builds bin/mwbcfg (needs Go)
ln -s "$PWD" ~/.config/omarchy/plugins/sippin45.mwb
omarchy plugin validate "$PWD"        # validate the project folder, not the symlink
```

The symlink makes this project the source of truth. Do not keep a second copy
in the plugins directory.

## Add to the bar

```sh
omarchy plugin enable sippin45.mwb --section right
omarchy restart shell            # needed the first time so the bar picks it up
```

Open it by clicking the icon, or `omarchy-shell sippin45.mwb toggle`.

Move it later with `omarchy bar move sippin45.mwb --section right`.

## Point it at a different config (testing)

The helper honours `MWB_CONFIG` (config path) and `MWBCFG_SYSTEMCTL` /
`MWBCFG_JOURNALCTL` (replacement commands). The shell does not set them, so by
default the plugin edits `~/.config/mwb/config.toml` and controls the real
service.

## Remove

```sh
omarchy plugin disable sippin45.mwb
rm ~/.config/omarchy/plugins/sippin45.mwb     # removes the symlink only
omarchy restart shell
```

Nothing else is installed: no `shell.json` keys, no files outside
`~/.config/omarchy/plugins/`.
