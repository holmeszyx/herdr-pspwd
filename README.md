# herdr-pspwd

Copy the focused [Herdr TUI](https://herdr.dev/tui/) pane's current working directory to the clipboard.

## Prepare

- Install Herdr TUI **0.9.5 or newer**, then check `herdr --version`.
- Install **Go 1.27 or newer**, as required by `go.mod`, then check `go version`. The first build needs access to Go's module proxy to download the TOML parser.
- Download this repository and open a terminal in its root directory.
- Use a shell that loads its profile normally, not with `-NoProfile` or `--norc`.

The clipboard backend currently supports Windows. Setup keeps shell discovery and platform-specific config paths separate so other OS backends can be added later. The commands below use the current manifest's executable name; building a `.exe` on another OS does not add clipboard support.

## Install

Run these commands in order, stopping if one fails:

```text
go build -o bin/herdr-pspwd.exe .
herdr plugin link .
herdr plugin action invoke zyx.pspwd.setup
```

`plugin link` registers this directory, but **does not build it**. Keep the directory in place while the plugin is linked.

Setup:

1. Adds a `prefix+y` keybinding to Herdr's config, unless a copy-cwd binding already exists.
2. Installs cwd-reporting hooks into the supported shells it finds.
3. Attempts `herdr server reload-config`. If no server is available, the config loads on the next launch.

Setup is explicit: builds, plugin linking, and server startup do not change shell profiles or keybindings. Review the code before running it. Setup writes files as your current user; administrator access is not needed.

If invocation reports that no server is running, start `herdr` and run the setup action from a pane. Alternatively, run the executable's `setup` command directly:

```text
./bin/herdr-pspwd.exe setup
```

**Open a new pane after setup.** Existing shells have not loaded the new hooks.

## Copy a directory

Focus a shell pane and change to another directory. Press your Herdr prefix, release it, then press `y`. With the default prefix, this is **Ctrl+B, then Y**, not all three keys together.

Paste into another application to check the copied path. You can also invoke the action from a Herdr pane:

```text
herdr plugin action invoke zyx.pspwd.copy-cwd
```

This plugin does not add an item to the pane right-click menu. Use the shortcut or action command.

## Keybindings

Keybindings belong in Herdr's config, not in `herdr-plugin.toml`:

| Platform | Default config path |
| --- | --- |
| Linux / macOS | `~/.config/herdr/config.toml` |
| Windows | `%APPDATA%\herdr\config.toml` |

`HERDR_CONFIG_PATH` overrides the default. Set it to an absolute path before running setup if you use a custom config. Runtime action commands run from the plugin directory, so relative overrides resolve there. `herdr --help` shows Herdr's resolved default path.

Setup appends this block without reformatting existing config:

```toml
# herdr-pspwd: copy the focused pane's working directory
[[keys.command]]
key = "prefix+y"
type = "plugin_action"
command = "zyx.pspwd.copy-cwd"
description = "copy pane cwd"
```

To choose another shortcut, edit `key` in that block, then run:

```text
herdr server reload-config
```

Setup preserves an existing copy-cwd binding, including a custom shortcut. Repeated setup does not duplicate the block. Before modifying an existing config file, it saves the first version as `config.toml.bak-herdr-pspwd`.

If `prefix+y` is assigned to another action or command, setup stops before changing the config or shell profiles. Keep your existing shortcut and add the copy-cwd block manually with another key, then rerun setup. Invalid TOML and layouts that cannot safely be extended, such as inline `keys` tables, also require a manual edit first.

## Shell cwd reporting

Herdr needs OSC 7 directory reports to track `cd` reliably. Setup installs hooks in:

| Shell | Profile |
| --- | --- |
| Git Bash | `~/.bashrc` |
| PowerShell 7 | `$PROFILE.CurrentUserCurrentHost` reported by `pwsh` |
| Windows PowerShell 5.1 | `$PROFILE.CurrentUserCurrentHost` reported by `powershell` |

These are the currently supported shell targets, not a promise of support for every shell. Zsh and Fish hooks are not included.

Hooks emit directory reports only inside Herdr (`HERDR_ENV=1`). Loading a profile twice does not add a second hook. Installing again replaces old managed blocks and puts the current hook last so it wraps prompts configured earlier, such as posh-git's.

Before changing an existing profile, the installer saves its first version as `<profile>.bak-herdr-pspwd`. Lines outside managed blocks are retained; trailing whitespace at the end of the file may be normalized. Backups are not overwritten.

To install or update only the hooks, without changing keybindings:

```text
herdr plugin action invoke zyx.pspwd.install-shell
```

The direct equivalent is `./bin/herdr-pspwd.exe shell install`.

## Check installation

```text
herdr plugin list
herdr plugin action list --plugin zyx.pspwd
herdr plugin log list --plugin zyx.pspwd
```

The plugin should be enabled and list `copy-cwd`, `setup`, `install-shell`, and `uninstall-shell`. If disabled, run `herdr plugin enable zyx.pspwd`.

In a newly opened pane, change directories, invoke `copy-cwd`, and paste the result. A correct path after `cd` checks both the plugin and shell reporting.

## Update

After updating the source in the linked directory, stop any active setup command and rerun:

```text
go build -o bin/herdr-pspwd.exe .
herdr plugin link .
herdr plugin action invoke zyx.pspwd.setup
```

Open a new pane afterward. Relinking refreshes the manifest actions; rebuilding refreshes the embedded shell hooks. `herdr plugin update` does not update locally linked plugins.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| Build fails | Check `go version`, dependency download access, and the current platform limitation above. |
| Executable not found | Build before linking; run from the repository root. |
| Plugin/action not found | Check `plugin list`, enable the plugin if needed, and relink after manifest changes. |
| Setup reports a shortcut conflict | Add the copy-cwd binding manually with an unused key, then rerun setup. |
| Setup reports a profile error | The config and other profiles may already be updated. Fix the reported file and rerun; successful steps are safe to repeat. |
| Reload is unavailable | Start Herdr, or run `herdr server reload-config` again and check its output. |
| Shortcut does nothing | Confirm which config is loaded, reload it, check the configured prefix, then invoke the action directly and inspect plugin logs. |
| Copied path stays at the old directory | Open a new pane and check that its shell loads the profile shown by setup. Hooks need a fresh prompt after `cd`. |
| No target pane | Invoke copy-cwd from Herdr. Running the executable directly outside Herdr has no focused-pane context. |

For built-in help: `./bin/herdr-pspwd.exe help`.

## Remove

Remove the hooks **before** unlinking:

```text
herdr plugin action invoke zyx.pspwd.uninstall-shell
```

Then remove only the copy-cwd `[[keys.command]]` block from Herdr's config and run:

```text
herdr server reload-config
herdr plugin unlink zyx.pspwd
```

Open new panes to stop using hooks already loaded by existing shells. Unlinking leaves the source directory and backups in place. Do not restore an old backup over newer unrelated edits; remove the managed blocks instead.

If the plugin is already unlinked, remove hooks with `./bin/herdr-pspwd.exe shell uninstall`.

## Development

```text
go vet ./...
go test ./...
```

Setup tests use temporary config/profile files and a fake Herdr process. They do not install hooks or bindings into your real environment.

References: [Herdr plugin manifest and lifecycle](https://herdr.dev/tui/docs/plugins/), [Herdr configuration](https://herdr.dev/tui/docs/configuration/).
