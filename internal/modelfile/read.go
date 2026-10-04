// Package modelfile reads a bounded artifact against its pre-open identity.
package modelfile

import (
	"errors"
	"io"
	"os"
)

// ReadChecked validates the opened descriptor before reading. Callers retain
// their own Lstat errors and format checks; a symlink is not a regular artifact.
func ReadChecked(name string, max int64, expected os.FileInfo) ([]byte, error) {
	if max <= 0 || max > 1<<20 || expected == nil || !expected.Mode().IsRegular() ||
		expected.Size() <= 0 || expected.Size() > max {
		return nil, errors.New("bounded regular model file required")
	}
	f, err := openRead(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(expected, opened) {
		return nil, errors.New("model file changed during bounded open")
	}
	if opened.Size() != expected.Size() {
		return nil, errors.New("model file extent changed")
	}
	raw, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) != expected.Size() || int64(len(raw)) > max {
		return nil, errors.New("model file extent changed")
	}
	return raw, nil
}
