//go:build linux || android

package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func captureStdout(fn func()) (string, error) {
	type readResult struct {
		data []byte
		err  error
	}

	stdoutFD := int(os.Stdout.Fd())
	savedStdoutFD, err := unix.Dup(stdoutFD)
	if err != nil {
		return "", fmt.Errorf("failed to duplicate stdout: %w", err)
	}
	defer unix.Close(savedStdoutFD)

	reader, writer, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout capture pipe: %w", err)
	}
	defer reader.Close()

	readDone := make(chan readResult, 1)
	go func() {
		captured, readErr := io.ReadAll(reader)
		readDone <- readResult{data: captured, err: readErr}
	}()

	if err := unix.Dup3(int(writer.Fd()), stdoutFD, 0); err != nil {
		writer.Close()
		result := <-readDone
		if result.err != nil {
			return "", fmt.Errorf("failed reading captured stdout after redirect error: %w", result.err)
		}
		return "", fmt.Errorf("failed to redirect stdout: %w", err)
	}

	fn()

	restoreErr := unix.Dup3(savedStdoutFD, stdoutFD, 0)
	closeErr := writer.Close()

	result := <-readDone

	if restoreErr != nil {
		return "", fmt.Errorf("failed to restore stdout: %w", restoreErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("failed to close stdout capture writer: %w", closeErr)
	}
	if result.err != nil {
		return "", fmt.Errorf("failed reading captured stdout: %w", result.err)
	}

	return string(result.data), nil
}
