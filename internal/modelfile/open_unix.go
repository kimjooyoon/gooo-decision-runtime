//go:build unix

package modelfile

import (
	"os"
	"syscall"
)

func openRead(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
