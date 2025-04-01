//go:build linux
// +build linux

package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func openSerial(filePath string) (io.ReadCloser, error) {
	// Open the serial port
	f, err := os.OpenFile(filePath, os.O_RDONLY|os.O_SYNC, 0644)
	if err != nil {
		return f, err
	}

	// Configure serial port settings
	termios, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	if err != nil {
		_ = f.Close()
		return f, fmt.Errorf("failed to get terminal attributes: %w", err)
	}

	// Set baud rate to 115200
	termios.Cflag = unix.B115200 | unix.CS8 | unix.CREAD | unix.CLOCAL
	termios.Iflag = 0 // No special input processing
	termios.Oflag = 0 // Raw output mode
	termios.Lflag = 0 // No line processing
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(int(f.Fd()), unix.TCSETS, termios); err != nil {
		_ = f.Close()
		return f, fmt.Errorf("failed to set terminal attributes: %w", err)
	}

	return f, nil
}
