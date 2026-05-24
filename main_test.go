package main

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

const testVLESSID = "00000000-0000-0000-0000-000000000000"

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

func TestIsVersionCommand(t *testing.T) {
	for _, args := range [][]string{
		{"Xray-Link-Json", "--version"},
		{"Xray-Link-Json", "version"},
	} {
		if !isVersionCommand(args) {
			t.Fatalf("expected %v to be a version command", args)
		}
	}

	for _, args := range [][]string{
		{"Xray-Link-Json"},
		{"Xray-Link-Json", "-"},
		{"Xray-Link-Json", "--version", "extra"},
	} {
		if isVersionCommand(args) {
			t.Fatalf("expected %v not to be a version command", args)
		}
	}
}

func TestFormatVersionInfoIncludesBundledXrayVersion(t *testing.T) {
	got := formatVersionInfo("v1.2.3", "abc123", "2026-05-12T00:00:00Z", "25.1.30")
	want := "Xray-Link-Json v1.2.3\ncommit=abc123\ndate=2026-05-12T00:00:00Z\nxray=25.1.30\n"
	if got != want {
		t.Fatalf("unexpected version info:\nwant %q\ngot  %q", want, got)
	}
}

func TestDecodeXrayVersion(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`{"success":true,"data":"25.1.30"}`))
	if got := decodeXrayVersion(encoded); got != "25.1.30" {
		t.Fatalf("expected decoded Xray version, got %q", got)
	}

	if got := decodeXrayVersion("26.3.27"); got != "26.3.27" {
		t.Fatalf("expected raw Xray version, got %q", got)
	}

	if got := decodeXrayVersion("not-base64"); got != "unknown" {
		t.Fatalf("expected unknown for invalid version payload, got %q", got)
	}
}

func TestFillEmptyOutboundTags(t *testing.T) {
	input := json.RawMessage(`{
		"outbounds": [
			{"protocol": "vless", "tag": ""},
			{"protocol": "vless", "tag": "", "sendThrough": "named-outbound"},
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
	if parsed.Outbounds[1].Tag != "" {
		t.Fatalf("expected named outbound tag to remain empty, got %q", parsed.Outbounds[1].Tag)
	}
	if parsed.Outbounds[2].Tag != "keep-me" {
		t.Fatalf("expected non-empty tag to be preserved, got %q", parsed.Outbounds[2].Tag)
	}
	if parsed.Outbounds[3].Tag != "" {
		t.Fatalf("expected missing tag to remain absent/empty, got %q", parsed.Outbounds[3].Tag)
	}
}

func TestStripJSONNullFieldsRecursivelyRemovesObjectNulls(t *testing.T) {
	input := json.RawMessage(`{
		"keep": false,
		"zero": 0,
		"empty": "",
		"drop": null,
		"nested": {
			"drop": null,
			"keep": "value"
		},
		"array": [
			{"drop": null, "keep": "value"},
			null
		]
	}`)

	output, err := stripJSONNullFields(input)
	if err != nil {
		t.Fatalf("stripJSONNullFields returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("failed to unmarshal cleaned output: %v", err)
	}
	if _, exists := parsed["drop"]; exists {
		t.Fatalf("expected top-level null field to be removed: %#v", parsed)
	}
	if parsed["keep"] != false || parsed["zero"] != float64(0) || parsed["empty"] != "" {
		t.Fatalf("expected non-null falsy values to be preserved: %#v", parsed)
	}
	nested := jsonObject(t, parsed["nested"])
	if _, exists := nested["drop"]; exists || nested["keep"] != "value" {
		t.Fatalf("unexpected nested cleanup result: %#v", nested)
	}
	array, ok := parsed["array"].([]any)
	if !ok || len(array) != 2 || array[1] != nil {
		t.Fatalf("expected array shape and null entries to be preserved: %#v", parsed["array"])
	}
	arrayObject := jsonObject(t, array[0])
	if _, exists := arrayObject["drop"]; exists || arrayObject["keep"] != "value" {
		t.Fatalf("unexpected array object cleanup result: %#v", arrayObject)
	}
}

func TestConvertShareLinkToXrayJSONDefaultsVLESSEncryption(t *testing.T) {
	data, err := convertShareLinkToXrayJSONData("vless://" + testVLESSID + "@example.com:443?type=tcp")
	if err != nil {
		t.Fatalf("convertShareLinkToXrayJSONData returned error: %v", err)
	}
	outbound := firstOutbound(t, data)
	settings := jsonObject(t, outbound["settings"])
	if settings["encryption"] != "none" {
		t.Fatalf("expected upstream VLESS encryption default none, got %#v", settings["encryption"])
	}
}

func TestConvertShareLinkToXrayJSONSocks(t *testing.T) {
	user := base64.StdEncoding.EncodeToString([]byte("alice:secret"))
	data, err := convertShareLinkToXrayJSONData("socks://" + user + "@127.0.0.1:10050#proxy")
	if err != nil {
		t.Fatalf("convertShareLinkToXrayJSONData returned error: %v", err)
	}
	outbound := firstOutbound(t, data)
	settings := jsonObject(t, outbound["settings"])
	if outbound["protocol"] != "socks" || settings["address"] != "127.0.0.1" || settings["port"] != float64(10050) {
		t.Fatalf("unexpected socks outbound: %#v", outbound)
	}
	if settings["user"] != "alice" || settings["pass"] != "secret" {
		t.Fatalf("unexpected socks auth settings: %#v", settings)
	}
}

func TestConvertShareLinkToXrayJSONNormalizesSocks5AndSocks5h(t *testing.T) {
	for _, scheme := range []string{"socks5", "socks5h"} {
		data, err := convertShareLinkToXrayJSONData(scheme + "://alice:secret@example.com:1080#proxy")
		if err != nil {
			t.Fatalf("convertShareLinkToXrayJSONData(%s) returned error: %v", scheme, err)
		}
		outbound := firstOutbound(t, data)
		settings := jsonObject(t, outbound["settings"])
		if outbound["protocol"] != "socks" || settings["address"] != "example.com" || settings["port"] != float64(1080) {
			t.Fatalf("unexpected normalized %s outbound: %#v", scheme, outbound)
		}
		if settings["user"] != "alice" || settings["pass"] != "secret" {
			t.Fatalf("unexpected normalized %s auth settings: %#v", scheme, settings)
		}
	}
}

func TestConvertBareHTTPProxyURLToXrayJSON(t *testing.T) {
	password := "very-long-hyphenated-password-value"
	data, ok, err := convertBareHTTPProxyURLToXrayJSON("http://proxyuser:" + password + "@example.com:2060#web")
	if err != nil {
		t.Fatalf("convertBareHTTPProxyURLToXrayJSON returned error: %v", err)
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

func TestConvertBareHTTPProxyURLToXrayJSONSkipsHTTPURLs(t *testing.T) {
	if _, ok, err := convertBareHTTPProxyURLToXrayJSON("http://example.com:8080/sub?token=abc"); ok || err != nil {
		t.Fatalf("expected non-bare http URL to be skipped, ok=%v err=%v", ok, err)
	}
}

func TestConvertShareLinkToXrayJSONRejectsInvalidSocks5Port(t *testing.T) {
	if _, err := convertShareLinkToXrayJSONData("socks5://127.0.0.1:70000"); err == nil {
		t.Fatal("expected invalid socks5 port error")
	}
}

func TestConvertShareLinkToXrayJSONRejectsMixedProxyURL(t *testing.T) {
	if _, err := convertShareLinkToXrayJSONData("mixed://127.0.0.1:1080"); err == nil {
		t.Fatal("expected mixed proxy URL error")
	}
}

func TestNormalizeShareTextForLibXrayTrimsTrailingVMessQRCodeJunk(t *testing.T) {
	payload := "eyJhZGQiOiI4NS4xOTQuMjQzLjExNyIsImFpZCI6IjAiLCJhbHBuIjoiIiwiZnAiOiIiLCJob3N0IjoiODUuMTk0LjI0My4xMTciLCJpZCI6IjE0MjNmYTU0LTEyNTYtNGNiYi05NjYzLWUwY2NhYjQ2ZjAyYSIsImluc2VjdXJlIjoiMCIsIm5ldCI6IndzIiwicGF0aCI6Ii92bWVzcy1hcmdvIiwicG9ydCI6IjU3ODc4IiwicHMiOiJJUi1AVjJyYXlBbHBoYS0yNyIsInNjeSI6ImF1dG8iLCJzbmkiOiIiLCJ0bHMiOiIiLCJ0eXBlIjoiLS0tIiwidiI6IjIifQ=="
	input := "vmess://" + payload + "vmess"
	want := "vmess://" + payload
	got, err := normalizeShareTextForLibXray(input)
	if err != nil {
		t.Fatalf("normalizeShareTextForLibXray returned error: %v", err)
	}
	if got != want {
		t.Fatalf("expected trailing VMess QR junk to be trimmed\nwant %q\ngot  %q", want, got)
	}
}

func TestNormalizeShareTextForLibXrayKeepsOnlyURILinesWhenPresent(t *testing.T) {
	input := "# header\n\nnot a link\nhttps://example.com/page\nvmess://abc\nvless://id@example.com:443\nhttp://proxyuser:password@example.com:2060\n"
	want := "vless://id@example.com:443\nhttp://proxyuser:password@example.com:2060"
	got, err := normalizeShareTextForLibXray(input)
	if err != nil {
		t.Fatalf("normalizeShareTextForLibXray returned error: %v", err)
	}
	if got != want {
		t.Fatalf("expected subscription comments and prose to be filtered\nwant %q\ngot  %q", want, got)
	}
}

func TestConvertShareLinkToXrayJSONPreservesXHTTP(t *testing.T) {
	link := "vless://" + testVLESSID + "@203.0.113.10:443?type=xhttp&encryption=none&path=%2Fxhttp-test&host=edge.example.com&mode=auto&security=tls&alpn=h2%2Chttp%2F1.1&sni=edge.example.com#xhttp-test"
	data, err := convertShareLinkToXrayJSONData(link)
	if err != nil {
		t.Fatalf("convertShareLinkToXrayJSONData returned error: %v", err)
	}
	if strings.Contains(string(data), ":null") {
		t.Fatalf("expected converted JSON to omit null fields, got %s", data)
	}

	streamSettings := jsonObject(t, firstOutbound(t, data)["streamSettings"])
	if streamSettings["network"] != "xhttp" || streamSettings["security"] != "tls" {
		t.Fatalf("unexpected stream settings: %#v", streamSettings)
	}
	if _, exists := streamSettings["grpcSettings"]; exists {
		t.Fatalf("expected null grpcSettings to be omitted: %#v", streamSettings)
	}
	xhttpSettings := jsonObject(t, streamSettings["xhttpSettings"])
	if xhttpSettings["path"] != "/xhttp-test" || xhttpSettings["host"] != "edge.example.com" || xhttpSettings["mode"] != "auto" {
		t.Fatalf("unexpected xhttp settings: %#v", xhttpSettings)
	}
	tlsSettings := jsonObject(t, streamSettings["tlsSettings"])
	if tlsSettings["serverName"] != "edge.example.com" {
		t.Fatalf("unexpected TLS settings: %#v", tlsSettings)
	}
	alpn, ok := tlsSettings["alpn"].([]any)
	if !ok || len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1" {
		t.Fatalf("unexpected ALPN settings: %#v", tlsSettings["alpn"])
	}
}

func TestConvertXrayJSONToShareTextPreservesXHTTP(t *testing.T) {
	linkText, err := convertXrayJSONToShareText(`{
		"outbounds": [
			{
				"protocol": "vless",
				"tag": "xhttp-test",
				"settings": {
					"vnext": [
						{
							"address": "203.0.113.10",
							"port": 443,
							"users": [
								{
									"id": "` + testVLESSID + `",
									"encryption": "none"
								}
							]
						}
					]
				},
				"streamSettings": {
					"network": "xhttp",
					"security": "tls",
					"tlsSettings": {
						"serverName": "edge.example.com",
						"alpn": ["h2", "http/1.1"]
					},
					"xhttpSettings": {
						"path": "/xhttp-test",
						"host": "edge.example.com",
						"mode": "auto"
					}
				}
			}
		]
	}`)
	if err != nil {
		t.Fatalf("convertXrayJSONToShareText returned error: %v", err)
	}

	parsed, err := url.Parse(strings.TrimSpace(linkText))
	if err != nil {
		t.Fatalf("failed to parse generated link: %v", err)
	}
	query := parsed.Query()
	if parsed.Scheme != "vless" || query.Get("type") != "xhttp" || query.Get("path") != "/xhttp-test" || query.Get("host") != "edge.example.com" || query.Get("mode") != "auto" {
		t.Fatalf("generated link lost XHTTP values: %s", linkText)
	}
	if query.Get("security") != "tls" || query.Get("sni") != "edge.example.com" || query.Get("alpn") != "h2,http/1.1" {
		t.Fatalf("generated link lost TLS values: %s", linkText)
	}
}

func TestConvertShareLinkToXrayJSONPreservesHTTPUpgrade(t *testing.T) {
	link := "vless://" + testVLESSID + "@203.0.113.11:443?type=httpupgrade&encryption=none&path=%2Fupgrade&host=upgrade.example.com"
	data, err := convertShareLinkToXrayJSONData(link)
	if err != nil {
		t.Fatalf("convertShareLinkToXrayJSONData returned error: %v", err)
	}

	streamSettings := jsonObject(t, firstOutbound(t, data)["streamSettings"])
	upgradeSettings := jsonObject(t, streamSettings["httpupgradeSettings"])
	if upgradeSettings["path"] != "/upgrade" || upgradeSettings["host"] != "upgrade.example.com" {
		t.Fatalf("unexpected httpupgrade settings: %#v", upgradeSettings)
	}
}

func TestConvertShareLinkToXrayJSONPreservesSplitHTTP(t *testing.T) {
	link := "vless://" + testVLESSID + "@203.0.113.12:443?type=splithttp&encryption=none&path=%2Fsplit&host=split.example.com&mode=stream-up"
	data, err := convertShareLinkToXrayJSONData(link)
	if err != nil {
		t.Fatalf("convertShareLinkToXrayJSONData returned error: %v", err)
	}

	streamSettings := jsonObject(t, firstOutbound(t, data)["streamSettings"])
	if streamSettings["network"] != "splithttp" {
		t.Fatalf("unexpected stream settings: %#v", streamSettings)
	}
	xhttpSettings := jsonObject(t, streamSettings["xhttpSettings"])
	if xhttpSettings["path"] != "/split" || xhttpSettings["host"] != "split.example.com" || xhttpSettings["mode"] != "stream-up" {
		t.Fatalf("unexpected splithttp settings: %#v", xhttpSettings)
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

func firstOutbound(t *testing.T, data json.RawMessage) map[string]any {
	t.Helper()
	var parsed struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if len(parsed.Outbounds) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(parsed.Outbounds))
	}
	return parsed.Outbounds[0]
}

func jsonObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected JSON object, got %#v", value)
	}
	return object
}
