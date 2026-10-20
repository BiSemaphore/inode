package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestKind(t *testing.T) {
	for cat, want := range map[string]string{
		"credentials": "credential", "commands": "command", "runbooks": "runbook", "decisions": "decision",
		"references": "reference", "contacts": "contact", "snippets": "snippet", "learnings": "learning",
		"notes": "item", "": "item", "custom": "custom",
	} {
		if got := kind(cat); got != want {
			t.Errorf("kind(%q) = %q, want %q", cat, got, want)
		}
	}
}

// The old `inode note` spelling still runs the same code, hidden from help.
func TestNoteAliasIsHiddenAndShared(t *testing.T) {
	if !noteCmd.Hidden {
		t.Error("inode note should be hidden from help")
	}
	if showCmd.Use != "show <id>" || deleteCmd.Use != "delete <id>" {
		t.Errorf("unexpected commands: %q, %q", showCmd.Use, deleteCmd.Use)
	}
	var help bytes.Buffer
	rootCmd.SetOut(&help)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := help.String()
	for _, w := range []string{" note ", " notes", "note,", "notes,"} {
		if strings.Contains(out, w) {
			t.Errorf("inode --help still says %q:\n%s", w, out)
		}
	}
	for _, c := range []string{"show", "delete"} {
		if !strings.Contains(out, "\n  "+c+" ") {
			t.Errorf("inode --help does not list %q", c)
		}
	}
}
