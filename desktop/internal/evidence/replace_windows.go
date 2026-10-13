//go:build windows

package evidence

import (
	"errors"
	"syscall"
)

func retryableReplace(err error) bool {
	return errors.Is(err, syscall.Errno(5)) || errors.Is(err, syscall.Errno(32))
}
