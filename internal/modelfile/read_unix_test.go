//go:build unix

package modelfile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadCheckedReplacementFIFORejectsWithoutWriter(t *testing.T) {
	name := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(name, []byte("abcd"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(name, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := ReadChecked(name, 4, info); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("accepted FIFO replacement")
		}
	case <-time.After(time.Second):
		// Join this owned reader before TempDir cleanup, including a failing
		// blocking implementation. A timeout alone does not establish wait.
		writer, err := os.OpenFile(name, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Fatal("cannot release owned reader", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		<-done
		t.Fatal("opened FIFO blocked until owned writer release")
	}
}

func TestReadCheckedReplacementSymlinkRejected(t *testing.T) {
	name := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(name, []byte("abcd"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(name)
	if err != nil {
		t.Fatal(err)
	}
	actual := name + ".original"
	if err := os.Rename(name, actual); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, name); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadChecked(name, 4, info); err == nil {
		t.Fatal("accepted replacement symlink")
	}
}
