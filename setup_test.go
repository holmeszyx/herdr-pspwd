package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestMain(m *testing.M) {
	if os.Getenv("HERDR_PSPWD_TEST_PROCESS") == "1" {
		if len(os.Args) > 1 && os.Args[1] == "-NoProfile" {
			fmt.Println(os.Getenv("HERDR_PSPWD_TEST_PROFILE"))
			os.Exit(0)
		}
		if !reflect.DeepEqual(os.Args[1:], []string{"server", "reload-config"}) {
			fmt.Fprintln(os.Stderr, "unexpected Herdr arguments:", os.Args[1:])
			os.Exit(3)
		}
		fmt.Print(os.Getenv("HERDR_PSPWD_TEST_RESPONSE"))
		if os.Getenv("HERDR_PSPWD_TEST_FAIL") == "1" {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestHerdrConfigPath(t *testing.T) {
	for _, tc := range []struct {
		name, goos, override, appdata, want string
		wantErr                             bool
	}{
		{"override", "linux", filepath.Join(t.TempDir(), "custom.toml"), "", "", false},
		{"windows", "windows", "", "roaming", filepath.Join("roaming", "herdr", "config.toml"), false},
		{"missing appdata", "windows", "", "", "", true},
		{"linux", "linux", "", "", filepath.Join("home", ".config", "herdr", "config.toml"), false},
		{"macOS", "darwin", "", "", filepath.Join("home", ".config", "herdr", "config.toml"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string {
				return map[string]string{"HERDR_CONFIG_PATH": tc.override, "APPDATA": tc.appdata}[key]
			}
			got, err := herdrConfigPath(tc.goos, getenv, func() (string, error) { return "home", nil })
			want := tc.want
			if tc.override != "" {
				want = tc.override
			}
			if (err != nil) != tc.wantErr || got != want {
				t.Fatalf("got %q, %v; want %q, error=%v", got, err, want, tc.wantErr)
			}
		})
	}
	_, err := herdrConfigPath("linux", func(string) string { return "" }, func() (string, error) { return "", errors.New("no home") })
	if err == nil {
		t.Fatal("expected home resolution error")
	}
}

func TestAddKeybinding(t *testing.T) {
	for _, tc := range []struct {
		name, content      string
		wantErr, unchanged bool
	}{
		{"empty", "", false, false},
		{"unrelated comments", "# keep\n[theme]\nname = 'terminal' # keep too", false, false},
		{"CRLF", "[keys]\r\nprefix = 'ctrl+a'\r\n", false, false},
		{"existing", keybindingBlock, false, true},
		{"custom shortcut", strings.ReplaceAll(keybindingBlock, "prefix+y", "prefix+alt+y"), false, true},
		{"custom with occupied default", "[keys]\nhelp = 'prefix+y'\n" + strings.ReplaceAll(keybindingBlock, "prefix+y", "prefix+alt+y"), false, true},
		{"other command", "[[keys.command]]\nkey = 'prefix+y'\ntype = 'shell'\ncommand = 'echo hi'\n", true, false},
		{"built-in action", "[keys]\nhelp = 'prefix+y'\n", true, false},
		{"array action", "[keys]\nhelp = ['prefix+?', 'prefix+y']\n", true, false},
		{"conflict after existing", keybindingBlock + "\n[[keys.command]]\nkey = 'prefix+y'\ntype = 'shell'\ncommand = 'echo hi'\n", true, false},
		{"conflict before existing", "[keys]\nhelp = 'prefix+y'\n" + keybindingBlock, true, false},
		{"invalid TOML", "[keys\n", true, false},
		{"invalid commands", "[keys]\ncommand = 'not an array'\n", true, false},
		{"inline keys", "keys = { prefix = 'ctrl+a' }\n", true, false},
		{"explicit command array", "[keys]\ncommand = []\n", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := addKeybinding(tc.content)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v, want error=%v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if tc.unchanged && got != tc.content {
				t.Fatal("existing binding changed")
			}
			if !strings.HasPrefix(got, tc.content) {
				t.Fatal("original bytes changed")
			}
			var parsed map[string]any
			if err := toml.Unmarshal([]byte(got), &parsed); err != nil {
				t.Fatal(err)
			}
			repeated, _, err := addKeybinding(got)
			if err != nil || got != repeated {
				t.Fatalf("not idempotent: %v", err)
			}
			if tc.name == "CRLF" && strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
				t.Fatal("added non-CRLF lines")
			}
		})
	}
}

func TestInstallKeybindingFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "config.toml")
	if _, err := installKeybinding(path); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if _, err := installKeybinding(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".bak-herdr-pspwd"); !os.IsNotExist(err) {
		t.Fatal("no-op created a backup")
	}
	info, _ := os.Stat(path)
	if _, err := installKeybinding(path); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !info.ModTime().Equal(after.ModTime()) {
		t.Fatal("no-op rewrote config")
	}
	original := "# user config\n[theme]\nname = 'terminal'\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installKeybinding(path); err != nil {
		t.Fatal(err)
	}
	backup, _ := os.ReadFile(path + ".bak-herdr-pspwd")
	if string(backup) != original {
		t.Fatal("backup does not contain original")
	}
	if err := os.WriteFile(path, []byte("# different\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installKeybinding(path); err != nil {
		t.Fatal(err)
	}
	backup, _ = os.ReadFile(path + ".bak-herdr-pspwd")
	if string(backup) != original {
		t.Fatal("first backup overwritten")
	}
	conflict := "[keys]\nhelp = 'prefix+y'\n"
	if err := os.WriteFile(path, []byte(conflict), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installKeybinding(path); err == nil {
		t.Fatal("expected conflict")
	}
	got, _ := os.ReadFile(path)
	if string(got) != conflict || len(first) == 0 {
		t.Fatal("conflict changed file")
	}
}

func TestInstallKeybindingBackupFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	const original = "# keep unchanged\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".bak-herdr-pspwd", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := installKeybinding(path); err == nil {
		t.Fatal("expected backup failure")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != original {
		t.Fatalf("config changed on backup failure: %v", err)
	}
}

func TestConfigureSetup(t *testing.T) {
	for _, scenario := range []string{"success", "conflict", "shell failure", "reload failure", "no shells"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			profile := filepath.Join(dir, ".bashrc")
			targets := []shellTarget{{"test Bash", profile, testSnippet}}
			if scenario == "success" {
				if err := os.WriteFile(profile, []byte("# existing profile\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "conflict" {
				if err := os.WriteFile(path, []byte("[keys]\nhelp = 'prefix+y'\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "shell failure" {
				if err := os.WriteFile(profile, []byte(beginMarker+"\nunterminated\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				targets = append(targets, shellTarget{"other shell", filepath.Join(dir, "other-profile"), testSnippet})
			}
			if scenario == "no shells" {
				targets = nil
			}
			var out bytes.Buffer
			reloads := 0
			reload := func() error {
				reloads++
				if scenario == "reload failure" {
					return errors.New("no server")
				}
				return nil
			}
			err := configureSetup(path, targets, reload, &out)
			wantErr := scenario == "conflict" || scenario == "shell failure" || scenario == "no shells"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v; output=%s", err, &out)
			}
			if scenario == "conflict" {
				if reloads != 0 {
					t.Fatal("reloaded despite conflict")
				}
				if _, err := os.Stat(profile); !os.IsNotExist(err) {
					t.Fatal("modified profile despite config conflict")
				}
				return
			}
			if reloads != 1 {
				t.Fatal("did not attempt reload")
			}
			if scenario == "reload failure" && !strings.Contains(out.String(), "next Herdr launch") {
				t.Fatal("missing deferred reload instructions")
			}
			if scenario == "success" {
				backup, err := os.ReadFile(profile + ".bak-herdr-pspwd")
				if err != nil || string(backup) != "# existing profile\n" {
					t.Fatalf("missing original profile backup: %v", err)
				}
				first, _ := os.ReadFile(profile)
				if err := configureSetup(path, targets, reload, &out); err != nil {
					t.Fatal(err)
				}
				second, _ := os.ReadFile(profile)
				if !bytes.Equal(first, second) || !strings.Contains(out.String(), "up to date") {
					t.Fatal("repeated setup changed profile")
				}
			}
			if scenario == "shell failure" {
				other, err := os.ReadFile(filepath.Join(dir, "other-profile"))
				if err != nil || !strings.Contains(string(other), beginMarker) {
					t.Fatalf("failed profile prevented other profiles from installing: %v", err)
				}
				if strings.Contains(out.String(), "Setup complete.") {
					t.Fatal("reported success despite partial failure")
				}
			}
		})
	}
}

func TestManifestSetupActions(t *testing.T) {
	data, err := os.ReadFile("herdr-plugin.toml")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ID      string `toml:"id"`
		Actions []struct {
			ID       string   `toml:"id"`
			Contexts []string `toml:"contexts"`
			Command  []string `toml:"command"`
		} `toml:"actions"`
	}
	if err := toml.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "zyx.pspwd" {
		t.Fatalf("unexpected plugin ID: %s", manifest.ID)
	}
	want := map[string][]string{
		"copy-cwd":        {"./bin/herdr-pspwd.exe"},
		"setup":           {"./bin/herdr-pspwd.exe", "setup"},
		"install-shell":   {"./bin/herdr-pspwd.exe", "shell", "install"},
		"uninstall-shell": {"./bin/herdr-pspwd.exe", "shell", "uninstall"},
	}
	for _, action := range manifest.Actions {
		if !reflect.DeepEqual(action.Command, want[action.ID]) {
			t.Fatalf("unexpected command for %s: %v", action.ID, action.Command)
		}
		context := "global"
		if action.ID == "copy-cwd" {
			context = "pane"
		}
		if !reflect.DeepEqual(action.Contexts, []string{context}) {
			t.Fatalf("unexpected context for %s", action.ID)
		}
		delete(want, action.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing manifest actions: %v", want)
	}
}

func TestReloadHerdrConfig(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", exe)
	t.Setenv("HERDR_PSPWD_TEST_PROCESS", "1")
	for _, tc := range []struct {
		name, response, fail string
		wantErr              bool
	}{
		{"success", `{"result":{"ok":true}}`, "", false},
		{"API error", `{"error":{"code":"no_server"}}`, "", true},
		{"process failure", "no server", "1", true},
		{"bad response", "not JSON", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HERDR_PSPWD_TEST_RESPONSE", tc.response)
			t.Setenv("HERDR_PSPWD_TEST_FAIL", tc.fail)
			if err := reloadHerdrConfig(); (err != nil) != tc.wantErr {
				t.Fatalf("error=%v; want error=%v", err, tc.wantErr)
			}
		})
	}
}

func TestSetupCLI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("current application only builds on Windows")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	app := filepath.Join(dir, "herdr-pspwd.exe")
	if out, err := exec.Command("go", "build", "-o", app, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"pwsh.exe", "powershell.exe"} {
		if err := os.WriteFile(filepath.Join(dir, host), data, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "config.toml")
	t.Setenv("HERDR_PSPWD_TEST_PROCESS", "1")
	t.Setenv("HERDR_PSPWD_TEST_PROFILE", filepath.Join(dir, "profile.ps1"))
	t.Setenv("HERDR_PSPWD_TEST_RESPONSE", `{"result":{"ok":true}}`)
	t.Setenv("HERDR_CONFIG_PATH", path)
	t.Setenv("HERDR_BIN_PATH", exe)
	t.Setenv("HOME", dir)
	t.Setenv("PATH", dir)
	for range 2 {
		out, err := exec.Command(app, "setup").CombinedOutput()
		if err != nil || !bytes.Contains(out, []byte("Setup complete.")) {
			t.Fatalf("setup failed: %v\n%s", err, out)
		}
	}
	config, _ := os.ReadFile(path)
	if strings.Count(string(config), `command = "zyx.pspwd.copy-cwd"`) != 1 {
		t.Fatal("missing or duplicate copy binding")
	}
	profile, _ := os.ReadFile(filepath.Join(dir, "profile.ps1"))
	if strings.Count(string(profile), beginPrefix) != 1 {
		t.Fatal("missing or duplicate profile hook")
	}
}
