//go:build !windows

package main

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

func captureStdout(fn func()) (string, error) {
	type readResult struct {
		data []byte
		err  error
	}

	stdoutFD := int(os.Stdout.Fd())
	savedStdoutFD, err := syscall.Dup(stdoutFD)
	if err != nil {
		return "", fmt.Errorf("failed to duplicate stdout: %w", err)
	}
	defer syscall.Close(savedStdoutFD)

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

	if err := syscall.Dup2(int(writer.Fd()), stdoutFD); err != nil {
		writer.Close()
		result := <-readDone
		if result.err != nil {
			return "", fmt.Errorf("failed reading captured stdout after redirect error: %w", result.err)
		}
		return "", fmt.Errorf("failed to redirect stdout: %w", err)
	}

	fn()

	restoreErr := syscall.Dup2(savedStdoutFD, stdoutFD)
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
