package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParseArgsRequiresOneInput(t *testing.T) {
	opts, err := parseArgs([]string{"-"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if opts.Input != "-" {
		t.Fatalf("expected input '-', got %q", opts.Input)
	}

	if _, err := parseArgs([]string{}); err == nil {
		t.Fatal("expected error for missing input")
	}
	if _, err := parseArgs([]string{"first", "second"}); err == nil {
		t.Fatal("expected error for multiple inputs")
	}
}

func TestCallLibXrayWithStdoutRedirectMovesStdoutToStderr(t *testing.T) {
	var stderr bytes.Buffer
	previousStderr := stderrOut
	stderrOut = &stderr
	defer func() {
		stderrOut = previousStderr
	}()

	result, err := callLibXrayWithStdoutRedirect(func() string {
		fmt.Print("converter warning\n")
		return "encoded-result"
	})
	if err != nil {
		t.Fatalf("callLibXrayWithStdoutRedirect returned error: %v", err)
	}
	if result != "encoded-result" {
		t.Fatalf("expected encoded result, got %q", result)
	}
	if !strings.Contains(stderr.String(), "converter warning") {
		t.Fatalf("expected captured stdout to be written to stderr, got %q", stderr.String())
	}
}

func TestFillEmptyOutboundTags(t *testing.T) {
	input := json.RawMessage(`{
		"outbounds": [
			{"protocol": "vless", "tag": ""},
			{"protocol": "trojan", "tag": "keep-me"},
			{"protocol": "vmess"}
		]
	}`)

	output, count, err := fillEmptyOutboundTags(input, func() (string, error) {
		return "alpha-bravo", nil
	})
	if err != nil {
		t.Fatalf("fillEmptyOutboundTags returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 filled tag, got %d", count)
	}

	var parsed struct {
		Outbounds []struct {
			Tag string `json:"tag"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if parsed.Outbounds[0].Tag != "alpha-bravo" {
		t.Fatalf("expected first tag to be filled, got %q", parsed.Outbounds[0].Tag)
	}
	if parsed.Outbounds[1].Tag != "keep-me" {
		t.Fatalf("expected non-empty tag to be preserved, got %q", parsed.Outbounds[1].Tag)
	}
	if parsed.Outbounds[2].Tag != "" {
		t.Fatalf("expected missing tag to remain absent/empty, got %q", parsed.Outbounds[2].Tag)
	}
}

func TestNormalizeTagWord(t *testing.T) {
	if word, ok := normalizeTagWord("Lantern"); !ok || word != "lantern" {
		t.Fatalf("expected normalized word lantern, got %q ok=%v", word, ok)
	}
	for _, raw := range []string{"a", "two-words", "has'apostrophe", "123"} {
		if word, ok := normalizeTagWord(raw); ok {
			t.Fatalf("expected %q to be rejected, got %q", raw, word)
		}
	}
}
