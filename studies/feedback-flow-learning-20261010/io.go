package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
)

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error)         { require(err == nil, fmt.Sprint(err)) }
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func encoded(value any) []byte {
	raw, err := json.Marshal(value)
	must(err)
	return raw
}
func save(out, name string, value any) { saveRaw(out, name, encoded(value)) }
func saveRaw(out, name string, raw []byte) {
	f, err := os.OpenFile(filepath.Join(out, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	must(err)
	_, err = f.Write(raw)
	must(err)
	must(f.Close())
}
func rowFile(out, name string) *os.File {
	f, err := os.OpenFile(filepath.Join(out, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	must(err)
	return f
}
func appendRow(f *os.File, value any) { must(json.NewEncoder(f).Encode(value)) }
func gzipBytes(name string) ([]byte, string) {
	raw, err := os.ReadFile(name)
	must(err)
	z, err := gzip.NewReader(bytes.NewReader(raw))
	must(err)
	data, err := io.ReadAll(z)
	must(err)
	must(z.Close())
	return data, hash(raw)
}
func decodeLines[T any](raw []byte) []T {
	var rows []T
	for line := range bytes.SplitSeq(raw, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var row T
		must(json.Unmarshal(line, &row))
		rows = append(rows, row)
	}
	return rows
}
func producer() string {
	info, ok := debug.ReadBuildInfo()
	require(ok && info.GoVersion == "go1.27.2", "Go1.27.2 producer required")
	var revision string
	modified := true
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			revision = setting.Value
		}
		if setting.Key == "vcs.modified" {
			modified = setting.Value != "false"
		}
	}
	require(revision != "" && !modified, "clean committed producer required")
	return revision
}
func featureSHA(inputs [][384]float32, width int) string {
	h := sha256.New()
	_, _ = h.Write([]byte{byte(len(inputs))})
	var raw [1536]byte
	for _, input := range inputs {
		for i, value := range input[:width] {
			binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
		}
		_, _ = h.Write(raw[:width*4])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func choiceSHA(input [384]float32) string {
	var raw [1536]byte
	for i, value := range input {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
	}
	return hash(raw[:])
}
