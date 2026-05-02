/**
 * This program takes input from the command line and checks whether the input is a JSON or a link.
 * If the input is a JSON, it converts it to Share links, and if the input is a link, it converts it to JSON.
 * This is done using functions from the libXray library, and the result is encoded in Base64.
 * Programmer: NabiKAZ (x.com/NabiKAZ)
 * https://github.com/NabiKAZ/Xray-Link-Json
 */

package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/xtls/libxray"
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

func main() {
	stderrOut = newGrayTTYWriter(os.Stderr)
	log.SetOutput(stderrOut)
	log.SetFlags(0)

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

func decodeLibXrayResponse(encoded string) (*conversionEnvelope, error) {
	if encoded == "" {
		return nil, fmt.Errorf("empty conversion result")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 output: %w", err)
	}
	stderrln(prettyJSONOrRaw(decodedBytes))

	var envelope conversionEnvelope
	if err := json.Unmarshal(decodedBytes, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse conversion payload: %w", err)
	}

	if !envelope.Success {
		if envelope.Error != "" {
			return nil, fmt.Errorf("conversion failed: %s", envelope.Error)
		}
		return nil, fmt.Errorf("conversion failed")
	}

	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, fmt.Errorf("conversion returned empty data")
	}

	return &envelope, nil
}

func prettyJSONOrRaw(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err == nil {
		return buf.String()
	}
	return string(raw)
}

func convertShareLinkToXrayJSON(shareLink string) error {
	stderrln("Processing Share link...")

	shareLinkBase64 := base64.StdEncoding.EncodeToString([]byte(shareLink))
	encodedResult, err := callLibXrayWithStdoutRedirect(func() string {
		return libXray.ConvertShareLinksToXrayJson(shareLinkBase64)
	})
	if err != nil {
		return err
	}
	envelope, err := decodeLibXrayResponse(encodedResult)
	if err != nil {
		return err
	}

	data, filledTags, err := fillEmptyOutboundTags(envelope.Data, randomTwoWordTag)
	if err != nil {
		return err
	}
	if filledTags > 0 {
		stderrln("Filled empty outbound tags:", filledTags)
	}

	return writeDataToStdout(data)
}

func convertXrayJSONToShareLinks(xrayJSON string) error {
	stderrln("Processing Xray JSON...")

	encodedJSON := base64.StdEncoding.EncodeToString([]byte(xrayJSON))
	encodedResult, err := callLibXrayWithStdoutRedirect(func() string {
		return libXray.ConvertXrayJsonToShareLinks(encodedJSON)
	})
	if err != nil {
		return err
	}
	envelope, err := decodeLibXrayResponse(encodedResult)
	if err != nil {
		return err
	}

	var links string
	if err := json.Unmarshal(envelope.Data, &links); err == nil {
		fmt.Fprintln(os.Stdout, strings.TrimSpace(links))
		return nil
	}

	return writeDataToStdout(envelope.Data)
}

func callLibXrayWithStdoutRedirect(convert func() string) (string, error) {
	var encodedResult string
	capturedStdout, err := captureStdout(func() {
		encodedResult = convert()
	})
	if err != nil {
		return "", err
	}

	if capturedStdout != "" {
		if _, writeErr := fmt.Fprint(stderrOut, capturedStdout); writeErr != nil {
			return "", fmt.Errorf("failed writing captured diagnostics to stderr: %w", writeErr)
		}
		if !strings.HasSuffix(capturedStdout, "\n") {
			stderrln()
		}
	}

	return encodedResult, nil
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
