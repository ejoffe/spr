// +build windows

package terminal

import (
	"errors"
)

func Width() (int, error) {
	return 0, errors.New("unimplemented")
}
