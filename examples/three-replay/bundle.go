package main

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type bundle struct {
	archive *zip.ReadCloser
	files   map[string]pin
	members map[string]pin
}

func sha(b []byte) string { x := sha256.Sum256(b); return hex.EncodeToString(x[:]) }
func filePin(path string) (pin, error) {
	f, err := os.Open(path)
	if err != nil {
		return pin{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return pin{hex.EncodeToString(h.Sum(nil)), n}, err
}
func checkPin(path string, want pin) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != want.Bytes || want.Bytes <= 0 || want.Bytes > 48<<20 {
		return errors.New("bounded regular artifact required")
	}
	got, err := filePin(path)
	if err != nil || got != want || want.Bytes <= 0 || len(want.SHA) != 64 {
		return errors.New("fixed artifact differs")
	}
	return nil
}
func openBundle(root string) (bundle, error) {
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil || sha(raw) != manifestSHA {
		return bundle{}, errors.New("registered bundle manifest required")
	}
	var m struct {
		Files    map[string]pin            `json:"files"`
		Archives map[string]map[string]pin `json:"archive_members"`
	}
	if err = json.Unmarshal(raw, &m); err != nil || len(m.Files) != 56 || len(m.Archives["arm64.zip"]) != 85 {
		return bundle{}, errors.New("closed bundle inventory required")
	}
	path := filepath.Join(root, "arm64.zip")
	if err = checkPin(path, m.Files["arm64.zip"]); err != nil {
		return bundle{}, err
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return bundle{}, err
	}
	seen := map[string]bool{}
	for _, f := range z.File {
		p, ok := m.Archives["arm64.zip"][f.Name]
		if !ok || seen[f.Name] || !filepath.IsLocal(f.Name) || p.Bytes < 0 || p.Bytes > 1<<20 || uint64(p.Bytes) != f.UncompressedSize64 {
			z.Close()
			return bundle{}, errors.New("fixed archive inventory differs")
		}
		seen[f.Name] = true
	}
	if len(seen) != 85 {
		z.Close()
		return bundle{}, errors.New("missing archive member")
	}
	return bundle{z, m.Files, m.Archives["arm64.zip"]}, nil
}

func (b bundle) visit(name string, fn func(frozenRow) error) error {
	for _, member := range b.archive.File {
		if member.Name != name {
			continue
		}
		f, err := member.Open()
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		bounded := &io.LimitedReader{R: io.TeeReader(f, h), N: b.members[name].Bytes + 1}
		z, err := gzip.NewReader(bounded)
		if err != nil {
			return err
		}
		defer z.Close()
		decoded := &io.LimitedReader{R: z, N: (16 << 20) + 1}
		scan := bufio.NewScanner(decoded)
		scan.Buffer(make([]byte, 32768), 1<<20)
		rows := 0
		for scan.Scan() {
			var row frozenRow
			if rows >= 512 || json.Unmarshal(scan.Bytes(), &row) != nil {
				return errors.New("invalid frozen row")
			}
			if err = fn(row); err != nil {
				return err
			}
			rows++
		}
		if scan.Err() != nil || rows != 512 || decoded.N == 0 || bounded.N != 1 || hex.EncodeToString(h.Sum(nil)) != b.members[name].SHA {
			return errors.New("complete gzip journal digest/extent/CRC required")
		}
		return nil
	}
	return errors.New("frozen journal missing")
}
