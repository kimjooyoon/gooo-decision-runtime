package modelfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReadCheckedRegularIdentityAndBounds(t *testing.T) {
	name := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(name, []byte("abcd"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(name)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadChecked(name, 4, info)
	if err != nil || !bytes.Equal(got, []byte("abcd")) {
		t.Fatal(string(got), err)
	}
	for _, max := range []int64{-1, 0, 3, 1<<20 + 1} {
		if _, err := ReadChecked(name, max, info); err == nil {
			t.Fatalf("accepted bound %d", max)
		}
	}
	if err := os.WriteFile(name, []byte("abcde"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadChecked(name, 8, info); err == nil {
		t.Fatal("accepted changed extent")
	}
	other := filepath.Join(filepath.Dir(name), "other")
	if err := os.WriteFile(other, []byte("abcd"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, name); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadChecked(name, 8, info); err == nil {
		t.Fatal("accepted changed identity")
	}
	if _, err := ReadChecked(name, 8, nil); err == nil {
		t.Fatal("accepted missing identity")
	}
}
