/**
 * This program takes input from the command line and checks whether the input is a JSON or a link.
 * If the input is a JSON, it converts it to Share links, and if the input is a link, it converts it to JSON.
 * This is done using functions from the libXray library, and the result is encoded in Base64.
 * Programmer: NabiKAZ (x.com/NabiKAZ)
 * https://github.com/NabiKAZ/Xray-Link-Json
 */

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/xtls/libxray"
)

type conversionEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

const (
	ansiGray  = "\x1b[90m"
	ansiReset = "\x1b[0m"
)

var stderrOut io.Writer = os.Stderr

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

	if len(os.Args) < 2 {
		log.Fatal("Please provide a Share link, Xray JSON, file path, or '-' for stdin")
	}

	rawArg := os.Args[1]
	input, source, err := resolveInput(rawArg)
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}

	stderrln("Input:", rawArg)
	if source != "arg" {
		stderrln("Input source:", source)
	}

	if isLikelyJSON(input) {
		if err := convertXrayJSONToShareLinks(input); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := convertShareLinkToXrayJSON(input); err != nil {
		log.Fatal(err)
	}
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

func isLikelyJSON(input string) bool {
	input = strings.TrimSpace(input)
	return strings.HasPrefix(input, "{") || strings.HasPrefix(input, "[")
}

func decodeLibXrayResponse(encoded string) (*conversionEnvelope, error) {
	if encoded == "" {
		return nil, fmt.Errorf("empty conversion result")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 output: %w", err)
	}
	stderrln(string(decodedBytes))

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

func convertShareLinkToXrayJSON(shareLink string) error {
	stderrln("Processing Share link...")

	shareLinkBase64 := base64.StdEncoding.EncodeToString([]byte(shareLink))
	encodedResult := libXray.ConvertShareLinksToXrayJson(shareLinkBase64)
	envelope, err := decodeLibXrayResponse(encodedResult)
	if err != nil {
		return err
	}

	return writeDataToStdout(envelope.Data)
}

func convertXrayJSONToShareLinks(xrayJSON string) error {
	stderrln("Processing Xray JSON...")

	encodedJSON := base64.StdEncoding.EncodeToString([]byte(xrayJSON))
	encodedResult := libXray.ConvertXrayJsonToShareLinks(encodedJSON)
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
