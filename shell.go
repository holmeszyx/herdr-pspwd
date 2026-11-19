package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed shell/bash.sh
var bashSnippet string

//go:embed shell/powershell.ps1
var powershellSnippet string

const (
	beginPrefix = "# >>> herdr-pspwd"
	endPrefix   = "# <<< herdr-pspwd"
	beginMarker = beginPrefix + " shell cwd reporting (managed by `herdr-pspwd shell install`; edits are overwritten) >>>"
	endMarker   = endPrefix + " <<<"
)

type shellTarget struct {
	name    string
	path    string
	snippet string
}

func runShell(args []string) error {
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		fmt.Print(usage)
		return nil
	}
	if len(args) != 1 || (args[0] != "install" && args[0] != "uninstall") {
		return errUsage
	}
	install := args[0] == "install"
	var failed []string
	for _, t := range shellTargets() {
		status, err := applyToFile(t.path, t.snippet, install)
		if err != nil {
			failed = append(failed, t.name)
			fmt.Printf("%-18s %s: error: %v\n", t.name, t.path, err)
			continue
		}
		fmt.Printf("%-18s %s: %s\n", t.name, t.path, status)
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed: %s", strings.Join(failed, ", "))
	}
	fmt.Println("Open a new pane (or restart the shell) to load the change.")
	return nil
}

func shellTargets() []shellTarget {
	var targets []shellTarget
	if home := bashHome(); home != "" {
		rc := filepath.Join(home, ".bashrc")
		if fileExists(rc) || bashInstalled() {
			targets = append(targets, shellTarget{"Git Bash", rc, bashSnippet})
		}
	}
	seen := map[string]bool{}
	for _, host := range []string{"pwsh", "powershell"} {
		path := powershellProfile(host)
		if path == "" || seen[strings.ToLower(path)] {
			continue
		}
		seen[strings.ToLower(path)] = true
		targets = append(targets, shellTarget{host, path, powershellSnippet})
	}
	return targets
}

func bashHome() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	home, _ := os.UserHomeDir()
	return home
}

func bashInstalled() bool {
	if _, err := exec.LookPath("bash"); err == nil {
		return true
	}
	return fileExists(`C:\Program Files\Git\bin\bash.exe`)
}

func powershellProfile(host string) string {
	if _, err := exec.LookPath(host); err != nil {
		return ""
	}
	out, err := exec.Command(host, "-NoProfile", "-NoLogo", "-Command", "$PROFILE.CurrentUserCurrentHost").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func applyToFile(path, snippet string, install bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	exists := err == nil
	if !exists && !install {
		return "not present", nil
	}
	updated, err := applyBlock(string(data), snippet, install)
	if err != nil {
		return "", err
	}
	if updated == string(data) {
		if install {
			return "up to date", nil
		}
		return "not present", nil
	}

	mode := os.FileMode(0o644)
	if exists {
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
		// Keep the first backup: it holds the state from before any herdr-pspwd change.
		backup := path + ".bak-herdr-pspwd"
		if !fileExists(backup) {
			if err := os.WriteFile(backup, data, mode); err != nil {
				return "", fmt.Errorf("write backup: %w", err)
			}
		}
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(updated), mode); err != nil {
		return "", err
	}
	switch {
	case !install:
		return "removed", nil
	case strings.Contains(string(data), beginPrefix):
		return "updated", nil
	default:
		return "installed", nil
	}
}

// applyBlock removes every managed block and, when installing, appends the
// current one at the end of the file. The block goes last so it wraps prompts
// that earlier lines (such as posh-git) define. Lines outside the block keep
// their original bytes and line endings.
func applyBlock(content, snippet string, install bool) (string, error) {
	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}

	var kept strings.Builder
	inBlock := false
	for _, line := range strings.SplitAfter(content, "\n") {
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case !inBlock && strings.HasPrefix(trimmed, beginPrefix):
			inBlock = true
		case inBlock && strings.HasPrefix(trimmed, endPrefix):
			inBlock = false
		case !inBlock:
			kept.WriteString(line)
		}
	}
	if inBlock {
		return "", errors.New("found a herdr-pspwd start marker without an end marker; fix the file by hand")
	}

	out := strings.TrimRight(kept.String(), " \t\r\n")
	if !install {
		if out == "" {
			return "", nil
		}
		return out + eol, nil
	}

	body := strings.TrimRight(strings.ReplaceAll(snippet, "\r\n", "\n"), "\n")
	block := strings.Join(append(append([]string{beginMarker}, strings.Split(body, "\n")...), endMarker), eol) + eol
	if out == "" {
		return block, nil
	}
	return out + eol + eol + block, nil
}
