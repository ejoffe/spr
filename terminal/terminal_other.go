// +build !windows

package terminal

import (
	"os"

	"golang.org/x/sys/unix"
)

// Width returns the current character width of the terminal
func Width() (int, error) {
	terminalMaxSize, err := unix.IoctlGetWinsize(int(os.Stdin.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, err
	}
	return int(terminalMaxSize.Col), nil
}
