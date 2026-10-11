package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func digest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func read(root, name string, limit int64) ([]byte, error) {
	f, err := os.Open(filepath.Join(root, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("bounded regular %s required", name)
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("%s grew beyond bound", name)
	}
	return raw, nil
}
func corpus(root string) ([]source, []byte, error) {
	f, err := os.Open(filepath.Join(root, "corpus.json.gz"))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	g, err := gzip.NewReader(io.LimitReader(f, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	defer g.Close()
	raw, err := io.ReadAll(io.LimitReader(g, (8<<20)+1))
	if err != nil {
		return nil, nil, err
	}
	if len(raw) > 8<<20 {
		return nil, nil, fmt.Errorf("corpus exceeds 8MiB")
	}
	var sources []source
	if err = json.Unmarshal(raw, &sources); err != nil {
		return nil, nil, err
	}
	return sources, raw, nil
}
func decode(root, name string, out any) error {
	raw, err := read(root, name, 1<<20)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
