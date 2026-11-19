package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const copyAction = "zyx.pspwd.copy-cwd"

const keybindingBlock = `# herdr-pspwd: copy the focused pane's working directory
[[keys.command]]
key = "prefix+y"
type = "plugin_action"
command = "zyx.pspwd.copy-cwd"
description = "copy pane cwd"
`

func setup() error {
	path, err := herdrConfigPath(runtime.GOOS, os.Getenv, os.UserHomeDir)
	if err != nil {
		return err
	}
	return configureSetup(path, shellTargets(), reloadHerdrConfig, os.Stdout)
}

func herdrConfigPath(goos string, getenv func(string) string, homeDir func() (string, error)) (string, error) {
	if path := getenv("HERDR_CONFIG_PATH"); path != "" {
		return filepath.Abs(path)
	}
	if goos == "windows" {
		if appdata := getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "herdr", "config.toml"), nil
		}
		return "", errors.New("APPDATA is unset; set HERDR_CONFIG_PATH to Herdr's config.toml")
	}
	home, err := homeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "herdr", "config.toml"), nil
}

func configureSetup(path string, targets []shellTarget, reload func() error, out io.Writer) error {
	status, err := installKeybinding(path)
	if err != nil {
		return fmt.Errorf("keybinding setup (%s): %w; shell profiles were not changed, resolve it manually and rerun setup", path, err)
	}
	fmt.Fprintf(out, "Keybinding         %s: %s\n", path, status)
	shellErr := runShellTargets(targets, true, out)
	if err := reload(); err != nil {
		fmt.Fprintf(out, "Config reload unavailable: %v\nThe binding will load on the next Herdr launch. If Herdr is already running, run herdr server reload-config and check its output.\n", err)
	} else {
		fmt.Fprintln(out, "Herdr config reloaded.")
	}
	if shellErr != nil {
		return fmt.Errorf("keybinding is ready, but shell setup is incomplete: %w; fix the reported profiles and rerun setup", shellErr)
	}
	fmt.Fprintln(out, "Setup complete. Open a new pane, then use the binding shown above to copy cwd (prefix defaults to Ctrl+B).")
	return nil
}

func reloadHerdrConfig() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, herdrBin(), "server", "reload-config").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	// Herdr's API envelope can report an error even if the process exits cleanly.
	var response struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return fmt.Errorf("parse Herdr reload response: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("Herdr rejected config reload: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func installKeybinding(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	exists := err == nil
	updated, status, err := addKeybinding(string(data))
	if err != nil || updated == string(data) {
		return status, err
	}
	mode := os.FileMode(0o600)
	if exists {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		mode = info.Mode().Perm()
		backup, err := os.OpenFile(path+".bak-herdr-pspwd", os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil && !os.IsExist(err) {
			return "", fmt.Errorf("create backup: %w", err)
		}
		if os.IsExist(err) {
			info, statErr := os.Stat(path + ".bak-herdr-pspwd")
			if statErr != nil {
				return "", fmt.Errorf("check backup: %w", statErr)
			}
			if !info.Mode().IsRegular() {
				return "", errors.New("backup path is not a regular file")
			}
		}
		if err == nil {
			_, writeErr := backup.Write(data)
			closeErr := backup.Close()
			if err := errors.Join(writeErr, closeErr); err != nil {
				return "", fmt.Errorf("write backup: %w", err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(updated), mode); err != nil {
		return "", err
	}
	return status, nil
}

func addKeybinding(content string) (string, string, error) {
	var config struct {
		Keys struct {
			Command []struct {
				Key     string `toml:"key"`
				Type    string `toml:"type"`
				Command string `toml:"command"`
			} `toml:"command"`
		} `toml:"keys"`
	}
	if err := toml.Unmarshal([]byte(content), &config); err != nil {
		return "", "", fmt.Errorf("parse config: %w", err)
	}
	var raw map[string]any
	if err := toml.Unmarshal([]byte(content), &raw); err != nil {
		return "", "", err
	}
	existing := ""
	usesDefault := false
	var conflict error
	for _, command := range config.Keys.Command {
		isCopy := command.Type == "plugin_action" && command.Command == copyAction
		if usesCopyShortcut(command.Key) && !isCopy {
			conflict = errors.New("prefix+y is already used by another command")
		}
		if isCopy && command.Key != "" {
			existing = command.Key
			usesDefault = usesDefault || usesCopyShortcut(command.Key)
		}
	}
	if keys, ok := raw["keys"].(map[string]any); ok {
		for name, value := range keys {
			if name != "command" && usesCopyShortcut(value) {
				conflict = fmt.Errorf("prefix+y conflicts with keys.%s", name)
			}
		}
	}
	if conflict != nil && (existing == "" || usesDefault) {
		return "", "", conflict
	}
	if existing != "" {
		return content, fmt.Sprintf("already bound to %s (preserved)", existing), nil
	}
	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}
	updated := content
	if updated != "" && !strings.HasSuffix(updated, "\n") {
		updated += eol
	}
	if updated != "" {
		updated += eol
	}
	updated += strings.ReplaceAll(keybindingBlock, "\n", eol)
	// Inline tables and explicitly assigned arrays cannot always be extended.
	if err := toml.Unmarshal([]byte(updated), &config); err != nil {
		return "", "", fmt.Errorf("cannot safely append a binding to this TOML layout: %w", err)
	}
	return updated, "installed prefix+y", nil
}

func usesCopyShortcut(value any) bool {
	switch value := value.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "prefix+y")
	case []any:
		for _, item := range value {
			if usesCopyShortcut(item) {
				return true
			}
		}
	}
	return false
}
