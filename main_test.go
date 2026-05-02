package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParseArgsOutboundOnly(t *testing.T) {
	opts, err := parseArgs([]string{"--outbound-only", "-"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if !opts.OutboundOnly {
		t.Fatal("expected OutboundOnly to be true")
	}
	if opts.Input != "-" {
		t.Fatalf("expected input '-', got %q", opts.Input)
	}
}

func TestFormatOutboundsOnlyProducesPasteableSnippet(t *testing.T) {
	input := json.RawMessage(`{
		"log": {"loglevel": "warning"},
		"outbounds": [
			{"protocol": "vless", "tag": "first"},
			{"protocol": "trojan", "tag": "second"}
		]
	}`)

	snippet, err := formatOutboundsOnly(input)
	if err != nil {
		t.Fatalf("formatOutboundsOnly returned error: %v", err)
	}

	if strings.Contains(string(snippet), `"loglevel"`) {
		t.Fatalf("snippet included non-outbound fields: %s", snippet)
	}
	if strings.HasPrefix(strings.TrimSpace(string(snippet)), "[") {
		t.Fatalf("snippet should not include a wrapping array: %s", snippet)
	}

	pastedConfig := []byte(`{"outbounds":[` + string(snippet) + `]}`)
	if !json.Valid(pastedConfig) {
		t.Fatalf("snippet is not pasteable inside an outbounds array: %s", pastedConfig)
	}
}

func TestFormatOutboundsOnlyMissingField(t *testing.T) {
	_, err := formatOutboundsOnly(json.RawMessage(`{"log": {}}`))
	if err == nil {
		t.Fatal("expected error for missing outbounds field")
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
