package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func jsonBytes(value any) ([]byte, error) { return json.Marshal(value) }
func saveRaw(out, name string, raw []byte) error {
	f, err := os.OpenFile(filepath.Join(out, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
