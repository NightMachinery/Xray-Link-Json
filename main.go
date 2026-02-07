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
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/xtls/libxray"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide a Share link, Xray JSON, file path, or '-' for stdin")
	}

	rawArg := os.Args[1]
	input, source, err := resolveInput(rawArg)
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}

	fmt.Println("Input:", rawArg)
	if source != "arg" {
		fmt.Println("Input source:", source)
	}

	if isLikelyJSON(input) {
		convertXrayJsonToShareLinks(input)
	} else {
		convertShareLinkToXrayJson(input)
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

// Convert Share link to Xray JSON
func convertShareLinkToXrayJson(shareLink string) {
	fmt.Println("Processing Share link...")

	shareLinkBase64 := base64.StdEncoding.EncodeToString([]byte(shareLink))
	result := libXray.ConvertShareLinksToXrayJson(shareLinkBase64)

	if result == "" {
		log.Fatal("Error converting Share link to Xray JSON")
	} else {
		decodedBytes, err := base64.StdEncoding.DecodeString(result)
		if err != nil {
			log.Fatalf("Error decoding: %v", err)
		}

		fmt.Println("Decoded Xray JSON:")
		fmt.Println(string(decodedBytes))
	}
}

// Convert Xray JSON to Share links
func convertXrayJsonToShareLinks(xrayJson string) {
	fmt.Println("Processing Xray JSON...")

	encodedJson := base64.StdEncoding.EncodeToString([]byte(xrayJson))
	result := libXray.ConvertXrayJsonToShareLinks(encodedJson)

	if result == "" {
		log.Fatal("Error converting Xray JSON to Share links")
	} else {
		decodedBytes, err := base64.StdEncoding.DecodeString(result)
		if err != nil {
			log.Fatalf("Error decoding: %v", err)
		}

		fmt.Println("Decoded Share links:")
		fmt.Println(string(decodedBytes))
	}
}
