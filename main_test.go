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

func TestConvertBareProxyShareLinkToXrayJSONSocks(t *testing.T) {
	data, ok, err := convertBareProxyShareLinkToXrayJSON("socks5://127.0.0.1:10050")
	if err != nil {
		t.Fatalf("convertBareProxyShareLinkToXrayJSON returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected socks5 link to be handled")
	}

	var parsed struct {
		Outbounds []struct {
			Protocol string `json:"protocol"`
			Tag      string `json:"tag"`
			Settings struct {
				Servers []struct {
					Address string `json:"address"`
					Port    int    `json:"port"`
				} `json:"servers"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if len(parsed.Outbounds) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(parsed.Outbounds))
	}
	outbound := parsed.Outbounds[0]
	if outbound.Protocol != "socks" {
		t.Fatalf("expected socks protocol, got %q", outbound.Protocol)
	}
	if outbound.Tag != "socks" {
		t.Fatalf("expected default tag socks, got %q", outbound.Tag)
	}
	if len(outbound.Settings.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(outbound.Settings.Servers))
	}
	server := outbound.Settings.Servers[0]
	if server.Address != "127.0.0.1" || server.Port != 10050 {
		t.Fatalf("unexpected server: %#v", server)
	}
}

func TestConvertBareProxyShareLinkToXrayJSONWithUserAndTag(t *testing.T) {
	data, ok, err := convertBareProxyShareLinkToXrayJSON("socks5://alice:secret@example.com:1080#proxy")
	if err != nil {
		t.Fatalf("convertBareProxyShareLinkToXrayJSON returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected socks5 link to be handled")
	}

	var parsed struct {
		Outbounds []struct {
			Tag      string `json:"tag"`
			Settings struct {
				Servers []struct {
					Users []struct {
						User string `json:"user"`
						Pass string `json:"pass"`
					} `json:"users"`
				} `json:"servers"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	outbound := parsed.Outbounds[0]
	if outbound.Tag != "proxy" {
		t.Fatalf("expected fragment tag proxy, got %q", outbound.Tag)
	}
	users := outbound.Settings.Servers[0].Users
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].User != "alice" || users[0].Pass != "secret" {
		t.Fatalf("unexpected user: %#v", users[0])
	}
}

func TestConvertBareProxyShareLinkToXrayJSONHTTP(t *testing.T) {
	password := "very-long-hyphenated-password-value"
	data, ok, err := convertBareProxyShareLinkToXrayJSON("http://proxyuser:" + password + "@example.com:2060#web")
	if err != nil {
		t.Fatalf("convertBareProxyShareLinkToXrayJSON returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected bare http proxy link to be handled")
	}

	var parsed struct {
		Outbounds []struct {
			Protocol string `json:"protocol"`
			Tag      string `json:"tag"`
			Settings struct {
				Servers []struct {
					Address string `json:"address"`
					Port    int    `json:"port"`
					Users   []struct {
						User string `json:"user"`
						Pass string `json:"pass"`
					} `json:"users"`
				} `json:"servers"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	outbound := parsed.Outbounds[0]
	if outbound.Protocol != "http" {
		t.Fatalf("expected http protocol, got %q", outbound.Protocol)
	}
	if outbound.Tag != "web" {
		t.Fatalf("expected fragment tag web, got %q", outbound.Tag)
	}
	server := outbound.Settings.Servers[0]
	if server.Address != "example.com" || server.Port != 2060 {
		t.Fatalf("unexpected server: %#v", server)
	}
	if len(server.Users) != 1 || server.Users[0].User != "proxyuser" || server.Users[0].Pass != password {
		t.Fatalf("unexpected users: %#v", server.Users)
	}
}

func TestConvertBareProxyShareLinkToXrayJSONSkipsHTTPURLs(t *testing.T) {
	if _, ok, err := convertBareProxyShareLinkToXrayJSON("http://example.com:8080/sub?token=abc"); ok || err != nil {
		t.Fatalf("expected non-bare http URL to be skipped, ok=%v err=%v", ok, err)
	}
}

func TestConvertBareProxyShareLinkToXrayJSONRejectsInvalidPort(t *testing.T) {
	if _, ok, err := convertBareProxyShareLinkToXrayJSON("socks5://127.0.0.1:70000"); !ok || err == nil {
		t.Fatalf("expected invalid socks5 port error, ok=%v err=%v", ok, err)
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
