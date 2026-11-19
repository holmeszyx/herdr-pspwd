# herdr-pspwd

A [Herdr](https://herdr.dev) plugin that copies the focused pane's current working directory to the Windows clipboard, like `pwd | clip`.

Herdr 0.9.5 does not let plugins add items to the pane right-click menu, so the action is triggered by a keybinding (`prefix+y` below) or `herdr plugin action invoke zyx.pspwd.copy-cwd`.

## Install

```powershell
go build -o bin/herdr-pspwd.exe .
herdr plugin link F:\GoProject\herdr-pspwd
```

Add to `%APPDATA%\herdr\config.toml`, then run `herdr server reload-config`:

```toml
[[keys.command]]
key = "prefix+y"
type = "plugin_action"
command = "zyx.pspwd.copy-cwd"
description = "copy pane cwd"
```

## Shell cwd reporting (required)

Herdr does not see `cd` in Git Bash or PowerShell panes on its own. Each shell must report its directory with OSC 7 on every prompt. Set this up with one command:

```powershell
F:\GoProject\herdr-pspwd\bin\herdr-pspwd.exe shell install
# or, through Herdr:
herdr plugin action invoke zyx.pspwd.install-shell
```

It writes a managed block to every shell it finds:

- Git Bash: `~/.bashrc`
- PowerShell 7: `$PROFILE` reported by `pwsh`
- Windows PowerShell 5.1: `$PROFILE` reported by `powershell`

The command is safe to rerun:

- **No duplicates:** each run removes every `# >>> herdr-pspwd` ... `# <<< herdr-pspwd` block, then appends one current block at the end of the file. The block goes last so it wraps prompts set earlier, such as posh-git's.
- **Updates:** edit `shell/bash.sh` or `shell/powershell.ps1`, rebuild, and run `shell install` again. A file that is already current is left untouched (`up to date`).
- **Safety:** lines outside the block keep their bytes and line endings. Before the first change, the original file is copied to `<file>.bak-herdr-pspwd`.
- **Removal:** `herdr-pspwd.exe shell uninstall` removes the blocks.

The hooks only run inside Herdr (`HERDR_ENV=1`), and sourcing a profile twice does not add a second hook. Open a new pane after installing.
## Debugging

```powershell
herdr plugin log list --plugin zyx.pspwd
```
