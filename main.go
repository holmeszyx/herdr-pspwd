package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type paneInfo struct {
	CWD           string `json:"cwd"`
	ForegroundCWD string `json:"foreground_cwd"`
}

const usage = `herdr-pspwd copies a Herdr pane's current working directory to the clipboard.

Usage:
  herdr-pspwd                   Copy the focused pane's cwd to the clipboard
  herdr-pspwd shell install     Install or update shell cwd reporting
  herdr-pspwd shell uninstall   Remove shell cwd reporting
  herdr-pspwd help              Show this help (also -h, --help)

Copy:
  Normally run by Herdr as the plugin action zyx.pspwd.copy-cwd, for example
  through a prefix+y keybinding. The target pane comes from focused_pane_id in
  HERDR_PLUGIN_CONTEXT_JSON, or from HERDR_PANE_ID. The cwd is read with
  "herdr pane get" (HERDR_BIN_PATH, or herdr on PATH).

Shell cwd reporting:
  Herdr only sees "cd" when the shell reports its directory with OSC 7.
  "shell install" writes a managed block to every shell it finds:
    Git Bash                 ~/.bashrc
    PowerShell 7             $PROFILE of pwsh
    Windows PowerShell 5.1   $PROFILE of powershell
  Rerunning it replaces existing blocks instead of adding duplicates, and
  updates them after a new build. The first change to a file saves the
  original as <file>.bak-herdr-pspwd. Open a new pane afterwards.

Examples:
  herdr plugin action invoke zyx.pspwd.copy-cwd
  herdr plugin action invoke zyx.pspwd.install-shell
  herdr-pspwd shell install
`

var errUsage = errors.New("invalid usage")

func main() {
	if len(os.Args) > 1 {
		os.Exit(runCommand(os.Args[1:]))
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-pspwd:", err)
		notify("Copy cwd failed", err.Error())
		os.Exit(1)
	}
}

func runCommand(args []string) int {
	switch args[0] {
	case "help", "-h", "--help", "/?":
		fmt.Print(usage)
		return 0
	case "shell":
		err := runShell(args[1:])
		if err == nil {
			return 0
		}
		if errors.Is(err, errUsage) {
			fmt.Fprint(os.Stderr, "herdr-pspwd: expected \"shell install\" or \"shell uninstall\"\n\n"+usage)
			return 2
		}
		fmt.Fprintln(os.Stderr, "herdr-pspwd:", err)
		return 1
	default:
		fmt.Fprintf(os.Stderr, "herdr-pspwd: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
func run() error {
	paneID := targetPaneID(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"), os.Getenv("HERDR_PANE_ID"))
	if paneID == "" {
		return errors.New("no target pane in plugin context")
	}
	out, err := exec.Command(herdrBin(), "pane", "get", paneID).Output()
	if err != nil {
		return fmt.Errorf("herdr pane get %s: %w", paneID, err)
	}
	info, err := parsePaneGet(out)
	if err != nil {
		return err
	}
	cwd := pickCWD(info)
	if cwd == "" {
		return fmt.Errorf("pane %s has no known cwd", paneID)
	}
	if err := writeClipboard(cwd); err != nil {
		return fmt.Errorf("write clipboard: %w", err)
	}
	fmt.Println(cwd)
	notify("Copied cwd", cwd)
	return nil
}

func herdrBin() string {
	if bin := os.Getenv("HERDR_BIN_PATH"); bin != "" {
		return bin
	}
	return "herdr"
}

func notify(title, body string) {
	_ = exec.Command(herdrBin(), "notification", "show", title, "--body", body).Run()
}

func targetPaneID(contextJSON, envPaneID string) string {
	var ctx struct {
		FocusedPaneID string `json:"focused_pane_id"`
	}
	if contextJSON != "" && json.Unmarshal([]byte(contextJSON), &ctx) == nil && ctx.FocusedPaneID != "" {
		return ctx.FocusedPaneID
	}
	return envPaneID
}

func parsePaneGet(data []byte) (paneInfo, error) {
	var resp struct {
		Result struct {
			Pane *paneInfo `json:"pane"`
		} `json:"result"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return paneInfo{}, fmt.Errorf("parse pane get output: %w", err)
	}
	if resp.Error != nil {
		return paneInfo{}, fmt.Errorf("herdr error %s: %s", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.Pane == nil {
		return paneInfo{}, errors.New("pane get output has no pane")
	}
	return *resp.Result.Pane, nil
}

func pickCWD(info paneInfo) string {
	if cwd := cleanPath(info.CWD); cwd != "" {
		return cwd
	}
	return cleanPath(info.ForegroundCWD)
}

// cleanPath trims trailing separators but keeps roots such as `C:\` and `/`.
func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	for len(p) > 1 && strings.ContainsAny(p[len(p)-1:], `\/`) {
		if len(p) == 3 && p[1] == ':' {
			break
		}
		p = p[:len(p)-1]
	}
	return p
}
