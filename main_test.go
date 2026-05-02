package main

import (
	"bytes"
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
