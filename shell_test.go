package main

import (
	"strings"
	"testing"
)

const testSnippet = "echo hook\n"

func TestApplyBlockInstallIntoEmpty(t *testing.T) {
	got, err := applyBlock("", testSnippet, true)
	if err != nil {
		t.Fatal(err)
	}
	want := beginMarker + "\necho hook\n" + endMarker + "\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyBlockIsIdempotent(t *testing.T) {
	first, err := applyBlock("alias ll='ls -l'\n", testSnippet, true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := applyBlock(first, testSnippet, true)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("second install changed file:\n%q\n%q", first, second)
	}
	if strings.Count(second, beginPrefix) != 1 {
		t.Fatalf("expected one block, got %q", second)
	}
}

func TestApplyBlockReplacesLegacyAndDuplicateBlocks(t *testing.T) {
	in := "a\r\n\r\n# >>> herdr-pspwd: report cwd to herdr via OSC 7 >>>\r\nold\r\n# <<< herdr-pspwd <<<\r\nb\n" +
		beginMarker + "\r\nold2\r\n" + endMarker + "\r\n\r\n"
	got, err := applyBlock(in, "new\n", true)
	if err != nil {
		t.Fatal(err)
	}
	want := "a\r\n\r\nb\r\n\r\n" + beginMarker + "\r\nnew\r\n" + endMarker + "\r\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyBlockMovesBlockToEnd(t *testing.T) {
	in := "a\n" + beginMarker + "\nold\n" + endMarker + "\nImport-Module posh-git\n"
	got, err := applyBlock(in, "new\n", true)
	if err != nil {
		t.Fatal(err)
	}
	want := "a\nImport-Module posh-git\n\n" + beginMarker + "\nnew\n" + endMarker + "\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyBlockKeepsMixedLineEndingsOutsideBlock(t *testing.T) {
	in := "one\r\ntwo\nthree\r\n"
	got, err := applyBlock(in, "x\n", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "one\r\ntwo\nthree\r\n") {
		t.Fatalf("user lines changed: %q", got)
	}
}

func TestApplyBlockUninstall(t *testing.T) {
	installed, _ := applyBlock("a\n", testSnippet, true)
	got, err := applyBlock(installed, testSnippet, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\n" {
		t.Fatalf("got %q", got)
	}
	got, _ = applyBlock("a\n", testSnippet, false)
	if got != "a\n" {
		t.Fatalf("uninstall without block changed file: %q", got)
	}
}

func TestApplyBlockRejectsUnterminatedBlock(t *testing.T) {
	if _, err := applyBlock("a\n"+beginMarker+"\nstuff\n", testSnippet, true); err == nil {
		t.Fatal("expected error")
	}
}

func TestEmbeddedSnippets(t *testing.T) {
	for name, s := range map[string]string{"bash": bashSnippet, "powershell": powershellSnippet} {
		if !strings.Contains(s, "]7;file:///") {
			t.Errorf("%s snippet does not emit OSC 7", name)
		}
		if strings.Contains(s, beginPrefix) || strings.Contains(s, endPrefix) {
			t.Errorf("%s snippet must not contain block markers", name)
		}
	}
}
