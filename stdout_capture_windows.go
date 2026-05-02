//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
)

func captureStdout(fn func()) (string, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout capture pipe: %w", err)
	}
	defer reader.Close()

	previousStdout := os.Stdout
	os.Stdout = writer
	fn()
	os.Stdout = previousStdout

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close stdout capture writer: %w", err)
	}

	captured, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed reading captured stdout: %w", err)
	}
	return string(captured), nil
}
