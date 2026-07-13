/**
 * This program takes input from the command line and checks whether the input is a JSON or a link.
 * If the input is a JSON, it converts it to Share links, and if the input is a link, it converts it to JSON.
 * This is done using functions from the libXray library.
 * Programmer: NabiKAZ (x.com/NabiKAZ)
 * https://github.com/NabiKAZ/Xray-Link-Json
 */

package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/xtls/libxray"
	"github.com/xtls/libxray/share"
	xraylog "github.com/xtls/xray-core/common/log"
)

type conversionEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

type cliOptions struct {
	Input string
}

const (
	ansiGray       = "\x1b[38;5;245m"
	ansiReset      = "\x1b[0m"
	systemDictPath = "/usr/share/dict"
)

var stderrOut io.Writer = os.Stderr
var cachedTagWords []string

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type grayTTYWriter struct {
	target      io.Writer
	shouldColor bool
}

func newGrayTTYWriter(f *os.File) *grayTTYWriter {
	info, err := f.Stat()
	isTTY := err == nil && (info.Mode()&os.ModeCharDevice) != 0
	return &grayTTYWriter{target: f, shouldColor: isTTY}
}

func (w *grayTTYWriter) Write(p []byte) (int, error) {
	if !w.shouldColor || len(p) == 0 {
		return w.target.Write(p)
	}

	colored := make([]byte, 0, len(ansiGray)+len(p)+len(ansiReset))
	colored = append(colored, ansiGray...)
	colored = append(colored, p...)
	colored = append(colored, ansiReset...)

	written, err := w.target.Write(colored)
	if err != nil {
		if written <= len(ansiGray) {
			return 0, err
		}
		visible := written - len(ansiGray)
		if visible > len(p) {
			visible = len(p)
		}
		return visible, err
	}

	return len(p), nil
}

func stderrln(a ...any) {
	fmt.Fprintln(stderrOut, a...)
}

type xrayStderrLogWriter struct{}

func (xrayStderrLogWriter) Write(s string) error {
	stderrln(strings.TrimRight(s, "\r\n"))
	return nil
}

func (xrayStderrLogWriter) Close() error {
	return nil
}

func redirectXrayLoggerToStderr() {
	xraylog.RegisterHandler(xraylog.NewLogger(func() xraylog.Writer {
		return xrayStderrLogWriter{}
	}))
}

func main() {
	stderrOut = newGrayTTYWriter(os.Stderr)
	log.SetOutput(stderrOut)
	log.SetFlags(0)
	redirectXrayLoggerToStderr()

	if isVersionCommand(os.Args) {
		fmt.Print(versionInfo())
		return
	}

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	rawArg := opts.Input
	input, source, err := resolveInput(rawArg)
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}

	stderrln("Input:", rawArg)
	if source != "arg" {
		stderrln("Input source:", source)
	}

	if normalizedJSON, ok, err := normalizeJSONInput(input); err != nil {
		log.Fatal(err)
	} else if ok {
		if err := convertXrayJSONToShareLinks(normalizedJSON); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := convertShareLinkToXrayJSON(input); err != nil {
		log.Fatal(err)
	}
}

func isVersionCommand(args []string) bool {
	return len(args) == 2 && (args[1] == "--version" || args[1] == "version")
}

func versionInfo() string {
	return formatVersionInfo(version, commit, date, xrayVersion())
}

func formatVersionInfo(appVersion, appCommit, buildDate, bundledXrayVersion string) string {
	return fmt.Sprintf(
		"Xray-Link-Json %s\ncommit=%s\ndate=%s\nxray=%s\n",
		appVersion,
		appCommit,
		buildDate,
		bundledXrayVersion,
	)
}

func xrayVersion() string {
	return decodeXrayVersion(libXray.XrayVersion())
}

func decodeXrayVersion(encoded string) string {
	raw := strings.TrimSpace(encoded)
	if raw == "" {
		return "unknown"
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		if isLikelyVersionString(raw) {
			return raw
		}
		return "unknown"
	}

	value := strings.TrimSpace(string(decoded))
	if value == "" {
		return "unknown"
	}

	var envelope conversionEnvelope
	if err := json.Unmarshal([]byte(value), &envelope); err == nil && envelope.Success {
		var xray string
		if err := json.Unmarshal(envelope.Data, &xray); err == nil {
			xray = strings.TrimSpace(xray)
			if xray != "" {
				return xray
			}
		}
	}

	return value
}

func isLikelyVersionString(value string) bool {
	if value == "" || strings.ContainsAny(value, "\r\n\t ") {
		return false
	}
	return strings.Contains(value, ".")
}

func parseArgs(args []string) (cliOptions, error) {
	var opts cliOptions
	if len(args) != 1 {
		return opts, fmt.Errorf("please provide exactly one Share link, Xray JSON, file path, or '-' for stdin")
	}

	opts.Input = args[0]
	return opts, nil
}

func resolveInput(arg string) (string, string, error) {
	if arg == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", "stdin", err
		}
		input := strings.TrimSpace(string(data))
		if input == "" {
			return "", "stdin", fmt.Errorf("stdin is empty")
		}
		return input, "stdin", nil
	}

	if info, err := os.Stat(arg); err == nil && !info.IsDir() {
		data, err := os.ReadFile(arg)
		if err != nil {
			return "", arg, err
		}
		input := strings.TrimSpace(string(data))
		if input == "" {
			return "", arg, fmt.Errorf("file is empty: %s", arg)
		}
		return input, arg, nil
	}

	return strings.TrimSpace(arg), "arg", nil
}

func normalizeJSONInput(input string) (string, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", false, nil
	}

	stripped, err := stripJSONComments(trimmed)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse JSON comments: %w", err)
	}

	candidate := strings.TrimSpace(stripped)
	if !strings.HasPrefix(candidate, "{") && !strings.HasPrefix(candidate, "[") {
		return "", false, nil
	}

	var raw json.RawMessage
	if err := json.Unmarshal([]byte(candidate), &raw); err != nil {
		return "", false, fmt.Errorf("invalid JSON input: %w", err)
	}

	return candidate, true, nil
}

func stripJSONComments(input string) (string, error) {
	var out strings.Builder
	out.Grow(len(input))

	inString := false
	escaped := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}

		if ch == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				i += 2
				for ; i < len(input) && input[i] != '\n'; i++ {
				}
				if i < len(input) {
					out.WriteByte(input[i])
				}
				continue
			}
			if next == '*' {
				i += 2
				closed := false
				for ; i < len(input); i++ {
					if input[i] == '\n' {
						out.WriteByte('\n')
					}
					if input[i] == '*' && i+1 < len(input) && input[i+1] == '/' {
						i++
						closed = true
						break
					}
				}
				if !closed {
					return "", fmt.Errorf("unterminated block comment")
				}
				continue
			}
		}

		out.WriteByte(ch)
	}

	if inString {
		return "", fmt.Errorf("unterminated string")
	}

	return out.String(), nil
}

func convertShareLinkToXrayJSON(shareLink string) error {
	stderrln("Processing Share link...")

	data, err := convertShareLinkToXrayJSONData(shareLink)
	if err != nil {
		return err
	}

	return writeDataToStdout(data)
}

func convertShareLinkToXrayJSONData(shareLink string) (json.RawMessage, error) {
	if proxyJSON, ok, err := convertBareHTTPProxyURLToXrayJSON(shareLink); ok || err != nil {
		return proxyJSON, err
	}

	normalizedShareLink, err := normalizeShareTextForLibXray(shareLink)
	if err != nil {
		return nil, err
	}
	config, err := share.ConvertShareLinksToXrayJson(normalizedShareLink)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Xray JSON output: %w", err)
	}
	data, filledTags, err := fillEmptyOutboundTags(data, randomTwoWordTag)
	if err != nil {
		return nil, err
	}
	if filledTags > 0 {
		stderrln("Filled empty outbound tags:", filledTags)
	}
	return stripJSONNullFields(data)
}

func convertBareHTTPProxyURLToXrayJSON(shareLink string) (json.RawMessage, bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(shareLink))
	if err != nil {
		return nil, false, nil
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return nil, false, nil
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, false, nil
	}
	if parsed.RawQuery != "" {
		return nil, false, nil
	}
	if parsed.Hostname() == "" {
		return nil, true, fmt.Errorf("%s proxy link is missing a host", parsed.Scheme)
	}
	if parsed.Port() == "" {
		return nil, true, fmt.Errorf("%s proxy link is missing a port", parsed.Scheme)
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, true, fmt.Errorf("invalid %s proxy port: %q", parsed.Scheme, parsed.Port())
	}

	server := map[string]any{
		"address": parsed.Hostname(),
		"port":    port,
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		password, hasPassword := parsed.User.Password()
		if username != "" || hasPassword {
			server["users"] = []map[string]string{
				{
					"user": username,
					"pass": password,
				},
			}
		}
	}

	tag := strings.TrimSpace(parsed.Fragment)
	if tag == "" {
		tag = "http"
	}

	root := map[string]any{
		"outbounds": []map[string]any{
			{
				"protocol": "http",
				"tag":      tag,
				"settings": map[string]any{
					"servers": []map[string]any{server},
				},
			},
		},
	}

	data, err := json.Marshal(root)
	if err != nil {
		return nil, true, fmt.Errorf("failed to encode http outbound: %w", err)
	}
	return data, true, nil
}

func normalizeShareTextForLibXray(text string) (string, error) {
	lines := strings.Split(text, "\n")
	shareLinks := make([]string, 0, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lines[i] = trimmed
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if normalized, ok, err := normalizeSubscriptionURILine(trimmed); err != nil {
			return "", err
		} else if ok {
			shareLinks = append(shareLinks, normalized)
		}
	}
	if len(shareLinks) > 0 {
		return strings.Join(shareLinks, "\n"), nil
	}
	return strings.Join(lines, "\n"), nil
}

func normalizeSubscriptionURILine(line string) (string, bool, error) {
	if !strings.Contains(line, "://") {
		return "", false, nil
	}

	parsed, err := url.Parse(line)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false, nil
	}

	if strings.EqualFold(parsed.Scheme, "vmess") {
		normalized := normalizeVMessQRCodeLink(line)
		if isVMessQRCodeLink(normalized) || hasURLPort(normalized) {
			return normalized, true, nil
		}
		return "", false, nil
	}

	if strings.EqualFold(parsed.Scheme, "http") {
		if (parsed.Path == "" || parsed.Path == "/") && parsed.RawQuery == "" && parsed.Port() != "" {
			return line, true, nil
		}
		return "", false, nil
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return "", false, nil
	}
	if strings.EqualFold(parsed.Scheme, "mixed") {
		return "", false, fmt.Errorf("mixed proxy URLs are unsupported because Xray mixed is inbound-only")
	}
	if isSocks5ProxyScheme(parsed.Scheme) {
		normalized, err := normalizeSocks5URLForLibXray(parsed)
		if err != nil {
			return "", false, err
		}
		return normalized, true, nil
	}
	if requiresURLPort(parsed.Scheme) && parsed.Port() == "" {
		return "", false, nil
	}
	return line, true, nil
}

func normalizeSocks5URLForLibXray(parsed *url.URL) (string, error) {
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("%s proxy link is missing a host", parsed.Scheme)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid %s proxy port: %q", parsed.Scheme, parsed.Port())
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("%s proxy links must not include a path", parsed.Scheme)
	}
	if parsed.RawQuery != "" {
		return "", fmt.Errorf("%s proxy links must not include a query string", parsed.Scheme)
	}

	normalized := &url.URL{
		Scheme:   "socks",
		Host:     parsed.Host,
		Fragment: parsed.Fragment,
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		password, hasPassword := parsed.User.Password()
		if username != "" || hasPassword {
			encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
			normalized.User = url.User(encoded)
		}
	}
	return normalized.String(), nil
}

func isSocks5ProxyScheme(scheme string) bool {
	return strings.EqualFold(scheme, "socks5") || strings.EqualFold(scheme, "socks5h")
}

func hasURLPort(line string) bool {
	parsed, err := url.Parse(line)
	return err == nil && parsed.Port() != ""
}

func requiresURLPort(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "vless", "trojan", "socks", "socks5", "socks5h", "ss":
		return true
	default:
		return false
	}
}

func normalizeVMessQRCodeLink(link string) string {
	const prefix = "vmess://"
	if !strings.HasPrefix(strings.ToLower(link), prefix) {
		return link
	}

	payload := link[len(prefix):]
	if isVMessQRCodePayload(payload) {
		return link
	}

	if normalized, ok := trimVMessQRCodePayload(payload); ok {
		return prefix + normalized
	}
	return link
}

func isVMessQRCodeLink(link string) bool {
	const prefix = "vmess://"
	return strings.HasPrefix(strings.ToLower(link), prefix) && isVMessQRCodePayload(link[len(prefix):])
}

func trimVMessQRCodePayload(payload string) (string, bool) {
	if padding := strings.IndexByte(payload, '='); padding >= 0 {
		end := padding + 1
		if end < len(payload) && payload[end] == '=' {
			end++
		}
		candidate := payload[:end]
		if isVMessQRCodePayload(candidate) {
			return candidate, true
		}
	}

	for end := len(payload) - 1; end > 0; end-- {
		candidate := payload[:end]
		if isVMessQRCodePayload(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func isVMessQRCodePayload(payload string) bool {
	decoded, err := decodeShareBase64(payload)
	if err != nil {
		return false
	}

	var qr map[string]json.RawMessage
	if err := json.Unmarshal(decoded, &qr); err != nil {
		return false
	}

	_, hasAddress := qr["add"]
	_, hasID := qr["id"]
	_, hasPort := qr["port"]
	return hasAddress && hasID && hasPort
}

func decodeShareBase64(text string) ([]byte, error) {
	normalized := strings.NewReplacer("-", "+", "_", "/").Replace(text)
	if missingPadding := len(normalized) % 4; missingPadding != 0 {
		normalized += strings.Repeat("=", 4-missingPadding)
	}
	return base64.StdEncoding.DecodeString(normalized)
}

func convertXrayJSONToShareLinks(xrayJSON string) error {
	stderrln("Processing Xray JSON...")

	shareText, err := convertXrayJSONToShareText(xrayJSON)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, strings.TrimSpace(shareText))
	return nil
}

func convertXrayJSONToShareText(xrayJSON string) (string, error) {
	normalizedJSON, err := normalizeXrayJSONForShareLinks(xrayJSON)
	if err != nil {
		return "", err
	}
	return share.ConvertXrayJsonToShareLinks([]byte(normalizedJSON))
}

func normalizeXrayJSONForShareLinks(xrayJSON string) (string, error) {
	var root any
	if err := json.Unmarshal([]byte(xrayJSON), &root); err != nil {
		return "", fmt.Errorf("failed to parse Xray JSON for share-link normalization: %w", err)
	}

	rootObject, ok := root.(map[string]any)
	if !ok {
		return xrayJSON, nil
	}
	rawOutbounds, ok := rootObject["outbounds"].([]any)
	if !ok {
		return xrayJSON, nil
	}

	for _, rawOutbound := range rawOutbounds {
		outbound, ok := rawOutbound.(map[string]any)
		if !ok {
			continue
		}
		normalizeOutboundStreamSettingsForShareLink(outbound)
		settings, ok := outbound["settings"].(map[string]any)
		if !ok {
			continue
		}
		protocol, _ := outbound["protocol"].(string)
		normalizeOutboundSettingsForShareLink(protocol, settings)
	}

	encoded, err := json.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("failed to encode Xray JSON after share-link normalization: %w", err)
	}
	return string(encoded), nil
}

func normalizeOutboundStreamSettingsForShareLink(outbound map[string]any) {
	streamSettings, ok := outbound["streamSettings"].(map[string]any)
	if !ok {
		return
	}
	security, _ := streamSettings["security"].(string)
	if !strings.EqualFold(security, "reality") {
		return
	}
	realitySettings, ok := streamSettings["realitySettings"].(map[string]any)
	if !ok {
		return
	}
	copySettingIfMissingFromKey(realitySettings, realitySettings, "password", "publicKey")
}

func normalizeOutboundSettingsForShareLink(protocol string, settings map[string]any) {
	switch strings.ToLower(protocol) {
	case "vless", "vmess":
		normalizeVNextSettingsForShareLink(protocol, settings)
	case "trojan", "shadowsocks", "socks", "http":
		normalizeServerSettingsForShareLink(protocol, settings)
	}
}

func normalizeVNextSettingsForShareLink(protocol string, settings map[string]any) {
	if settings["address"] != nil && settings["port"] != nil {
		return
	}
	server := firstObjectFromArray(settings["vnext"])
	if server == nil {
		return
	}
	copySettingIfMissing(settings, server, "address")
	copySettingIfMissing(settings, server, "port")

	user := firstObjectFromArray(server["users"])
	if user == nil {
		return
	}
	copySettingIfMissing(settings, user, "id")
	copySettingIfMissing(settings, user, "level")
	copySettingIfMissing(settings, user, "email")
	switch strings.ToLower(protocol) {
	case "vless":
		copySettingIfMissing(settings, user, "flow")
		copySettingIfMissing(settings, user, "encryption")
	case "vmess":
		if settings["security"] == nil {
			if security, ok := user["security"]; ok {
				settings["security"] = security
			} else if security, ok := user["encryption"]; ok {
				settings["security"] = security
			}
		}
	}
}

func normalizeServerSettingsForShareLink(protocol string, settings map[string]any) {
	if settings["address"] != nil && settings["port"] != nil {
		return
	}
	server := firstObjectFromArray(settings["servers"])
	if server == nil {
		return
	}
	copySettingIfMissing(settings, server, "address")
	copySettingIfMissing(settings, server, "port")
	copySettingIfMissing(settings, server, "level")
	copySettingIfMissing(settings, server, "email")

	switch strings.ToLower(protocol) {
	case "trojan":
		copySettingIfMissing(settings, server, "password")
		copySettingIfMissing(settings, server, "flow")
	case "shadowsocks":
		copySettingIfMissing(settings, server, "method")
		copySettingIfMissing(settings, server, "password")
	case "socks", "http":
		user := firstObjectFromArray(server["users"])
		if user != nil {
			copySettingIfMissing(settings, user, "user")
			copySettingIfMissing(settings, user, "pass")
		}
	}
}

func firstObjectFromArray(value any) map[string]any {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	object, ok := items[0].(map[string]any)
	if !ok {
		return nil
	}
	return object
}

func copySettingIfMissing(target, source map[string]any, key string) {
	copySettingIfMissingFromKey(target, source, key, key)
}

func copySettingIfMissingFromKey(target, source map[string]any, targetKey, sourceKey string) {
	if target[targetKey] != nil {
		return
	}
	if value, ok := source[sourceKey]; ok && value != nil {
		target[targetKey] = value
	}
}

func fillEmptyOutboundTags(data json.RawMessage, tagGenerator func() (string, error)) (json.RawMessage, int, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, 0, fmt.Errorf("failed to parse JSON output for tag generation: %w", err)
	}

	rawOutbounds, ok := root["outbounds"]
	if !ok || rawOutbounds == nil {
		return data, 0, nil
	}

	outbounds, ok := rawOutbounds.([]any)
	if !ok {
		return nil, 0, fmt.Errorf("outbounds is not a JSON array")
	}

	filled := 0
	for i, rawOutbound := range outbounds {
		outbound, ok := rawOutbound.(map[string]any)
		if !ok {
			return nil, 0, fmt.Errorf("outbound %d is not a JSON object", i)
		}

		tag, ok := outbound["tag"].(string)
		if !ok || tag != "" {
			continue
		}
		if outboundHasName(outbound) {
			continue
		}

		generatedTag, err := tagGenerator()
		if err != nil {
			return nil, 0, fmt.Errorf("failed to generate tag for outbound %d: %w", i, err)
		}
		outbound["tag"] = generatedTag
		filled++
	}

	if filled == 0 {
		return data, 0, nil
	}

	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to encode JSON output after tag generation: %w", err)
	}

	return encoded, filled, nil
}

func outboundHasName(outbound map[string]any) bool {
	for _, key := range []string{"sendThrough", "name"} {
		if value, ok := outbound[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func stripJSONNullFields(data json.RawMessage) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output for null cleanup: %w", err)
	}
	cleaned := stripNullFields(value)
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return nil, fmt.Errorf("failed to encode JSON output after null cleanup: %w", err)
	}
	return encoded, nil
}

func stripNullFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if child == nil {
				delete(typed, key)
				continue
			}
			typed[key] = stripNullFields(child)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = stripNullFields(child)
		}
		return typed
	default:
		return value
	}
}

func randomTwoWordTag() (string, error) {
	words, _ := tagWords()
	if len(words) == 0 {
		words = fallbackTagWords()
	}

	first, err := randomWord(words)
	if err != nil {
		return "", err
	}
	second, err := randomWord(words)
	if err != nil {
		return "", err
	}

	return first + "-" + second, nil
}

func tagWords() ([]string, error) {
	if cachedTagWords != nil {
		return cachedTagWords, nil
	}

	words, err := loadDictionaryWords(systemDictPath)
	if err != nil {
		stderrln("Warning: failed to read dictionary words:", err)
		words = fallbackTagWords()
	}
	cachedTagWords = words
	return cachedTagWords, err
}

func loadDictionaryWords(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := readDictionaryFile(filepath.Join(dir, entry.Name()), seen); err != nil {
			return nil, err
		}
	}

	words := make([]string, 0, len(seen))
	for word := range seen {
		words = append(words, word)
	}
	return words, nil
}

func readDictionaryFile(path string, seen map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if word, ok := normalizeTagWord(scanner.Text()); ok {
			seen[word] = struct{}{}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func normalizeTagWord(raw string) (string, bool) {
	word := strings.ToLower(strings.TrimSpace(raw))
	if len(word) < 3 || len(word) > 12 {
		return "", false
	}

	for _, r := range word {
		if !unicode.IsLetter(r) {
			return "", false
		}
	}

	return word, true
}

func randomWord(words []string) (string, error) {
	if len(words) == 0 {
		return "", fmt.Errorf("word list is empty")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	if err != nil {
		return "", err
	}
	return words[n.Int64()], nil
}

func fallbackTagWords() []string {
	return []string{
		"amber", "anchor", "breeze", "brook", "cedar", "cloud",
		"copper", "ember", "falcon", "field", "harbor", "lantern",
		"meadow", "orbit", "pebble", "river", "silver", "summit",
	}
}

func writeDataToStdout(data json.RawMessage) error {
	if len(data) == 0 {
		return fmt.Errorf("no data to print")
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		fmt.Fprintln(os.Stdout, strings.TrimSpace(asString))
		return nil
	}

	if _, err := os.Stdout.Write(data); err != nil {
		return fmt.Errorf("failed writing output: %w", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		fmt.Fprintln(os.Stdout)
	}

	return nil
}
