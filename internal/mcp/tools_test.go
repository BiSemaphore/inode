package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/shahid-io/inode/internal/model"
)

func TestContentFor_NonSensitive_ReturnsPlaintext(t *testing.T) {
	n := &model.Note{ContentPlain: "echo hello", IsSensitive: false}
	if got := contentFor(n, false); got != "echo hello" {
		t.Errorf("non-sensitive note should pass through plaintext, got %q", got)
	}
}

func TestContentFor_Sensitive_NoReveal_Masks(t *testing.T) {
	n := &model.Note{ContentPlain: "sk-ant-secret", IsSensitive: true}
	if got := contentFor(n, false); got != maskedContent {
		t.Errorf("sensitive + !reveal should mask, got %q", got)
	}
}

func TestContentFor_Sensitive_WithReveal_Returns(t *testing.T) {
	n := &model.Note{ContentPlain: "sk-ant-secret", IsSensitive: true}
	if got := contentFor(n, true); got != "sk-ant-secret" {
		t.Errorf("sensitive + reveal should return plaintext, got %q", got)
	}
}

func TestContentFor_Sensitive_NoPlaintext_DefensivelyMasks(t *testing.T) {
	// If something upstream skipped decryption, the plaintext field is empty.
	// We must not leak the empty string (which the LLM would interpret as
	// "no content") — masking is the safer default than ambiguity.
	n := &model.Note{ContentPlain: "", IsSensitive: true}
	if got := contentFor(n, true); got != maskedContent {
		t.Errorf("sensitive note with no plaintext should mask even with reveal=true, got %q", got)
	}
}

func TestFormatSearchResults_IncludesAllMetadata(t *testing.T) {
	notes := []*model.Note{
		{
			ID:           "dc9a928b0000000000000000",
			Summary:      "echo hello command",
			Category:     "commands",
			Tags:         []string{"bash", "demo"},
			ContentPlain: "echo hello",
			Distance:     0.123,
		},
	}
	out := formatSearchResults("hello", notes, false)
	for _, want := range []string{"dc9a928b", "echo hello command", "commands", "bash, demo", "0.123", "echo hello"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in formatted output:\n%s", want, out)
		}
	}
}

func TestFormatSearchResults_SensitiveMaskedWhenNoReveal(t *testing.T) {
	notes := []*model.Note{
		{
			ID:           "ab123456" + strings.Repeat("0", 24),
			Summary:      "stripe key",
			Category:     "credentials",
			ContentPlain: "sk_live_secret",
			IsSensitive:  true,
		},
	}
	out := formatSearchResults("stripe", notes, false)
	if strings.Contains(out, "sk_live_secret") {
		t.Errorf("plaintext leaked when reveal=false:\n%s", out)
	}
	if !strings.Contains(out, maskedContent) {
		t.Errorf("expected masked content marker in output:\n%s", out)
	}
	// Metadata should still flow through.
	if !strings.Contains(out, "stripe key") || !strings.Contains(out, "credentials") {
		t.Errorf("metadata stripped from masked note:\n%s", out)
	}
}

func TestFormatSearchResults_SensitiveRevealedWithReveal(t *testing.T) {
	notes := []*model.Note{
		{
			ID:           "ab123456" + strings.Repeat("0", 24),
			Summary:      "stripe key",
			Category:     "credentials",
			ContentPlain: "sk_live_secret",
			IsSensitive:  true,
		},
	}
	out := formatSearchResults("stripe", notes, true)
	if !strings.Contains(out, "sk_live_secret") {
		t.Errorf("plaintext missing when reveal=true:\n%s", out)
	}
}

func TestFormatNoteList_FlagsSensitive(t *testing.T) {
	notes := []*model.Note{
		{ID: "aaaaaaaa00000000", Summary: "regular note", Category: "notes"},
		{ID: "bbbbbbbb00000000", Summary: "stripe key", Category: "credentials", IsSensitive: true},
	}
	out := formatNoteList(notes)
	if !strings.Contains(out, "[sensitive]") {
		t.Errorf("sensitive marker missing from list output:\n%s", out)
	}
	// Non-sensitive note should NOT carry the marker on its line.
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "regular note") && strings.Contains(line, "[sensitive]") {
			t.Errorf("non-sensitive note wrongly flagged sensitive: %q", line)
		}
	}
}

func TestFormatNote_HeaderFieldsPresent(t *testing.T) {
	n := &model.Note{
		ID:           "abcd1234efgh5678",
		Summary:      "smoke test",
		Category:     "notes",
		Tags:         []string{"smoke"},
		ContentPlain: "hello world",
		CreatedAt:    time.Date(2026, 5, 13, 15, 30, 0, 0, time.UTC),
	}
	out := formatNote(n, false)
	for _, want := range []string{"ID:", "abcd1234efgh5678", "Summary:", "smoke test", "Category:", "Tags:", "Created:", "Content:", "hello world"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in formatted note:\n%s", want, out)
		}
	}
}

func TestFormatNote_SensitiveMaskedWhenNoReveal(t *testing.T) {
	n := &model.Note{
		ID:           "secret00",
		Summary:      "aws key",
		Category:     "credentials",
		ContentPlain: "AKIA-EXAMPLE-KEY",
		IsSensitive:  true,
	}
	out := formatNote(n, false)
	if strings.Contains(out, "AKIA-EXAMPLE-KEY") {
		t.Errorf("plaintext leaked from get_note response when reveal=false:\n%s", out)
	}
	if !strings.Contains(out, maskedContent) {
		t.Errorf("masked content marker missing:\n%s", out)
	}
}

func TestShortID(t *testing.T) {
	if got := shortID("abcd1234ef56"); got != "abcd1234" {
		t.Errorf("expected 8-char prefix, got %q", got)
	}
	if got := shortID("short"); got != "short" {
		t.Errorf("ID shorter than 8 chars should pass through, got %q", got)
	}
}
