//go:build ignore

package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type frozenSpecification struct {
	ID, Split, Language, Contract, Source          string
	Family, Parameter, Goal, Wording, Parity, Form int
	PartsSHA                                       string `json:"parts_sha256"`
	SourceSHA                                      string `json:"source_sha256"`
	Probes                                         [4]struct{ Input, Expected int64 }
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func check(err error) {
	if err != nil {
		panic(err)
	}
}
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func run() {
	require(len(os.Args) == 2, "one prefit directory required")
	root := os.Args[1]
	raw, err := os.ReadFile(filepath.Join(root, "specifications.json"))
	check(err)
	var rows []frozenSpecification
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	check(d.Decode(&rows))
	require(len(rows) == 1536, "1536 specifications required")
	ids, contracts, coverage := map[string]bool{}, map[string]bool{}, map[[7]int]bool{}
	parts, withoutIntents := map[[4]int]string{}, map[[5]int]string{}
	counts := map[string]int{}
	for _, row := range rows {
		require(!ids[row.ID], "duplicate id")
		ids[row.ID] = true
		contracts[row.Contract] = true
		require(row.Family >= 0 && row.Family < 16 && row.Parameter >= 0 && row.Parameter < 3 && row.Goal >= 0 && row.Goal < 2 && row.Wording >= 0 && row.Wording < 1 && row.Parity >= 0 && row.Parity < 4 && row.Form >= 0 && row.Form < 2, "factor range")
		language := 0
		if row.Language == "en" {
			language = 1
		} else {
			require(row.Language == "ko", "language")
		}
		key := [7]int{row.Family, row.Parameter, row.Goal, language, row.Wording, row.Parity, row.Form}
		require(!coverage[key], "duplicate factor combination")
		coverage[key] = true
		split := "evaluation"
		if (7*row.Family+3)%16 < 8 {
			split = "train"
		} else if (7*row.Family+3)%16 < 12 {
			split = "validation"
		}
		require(row.Split == split, "family split differs")
		counts[split]++
		require(row.Source == "sources/"+row.ID+".gooo", "source path")
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Source)))
		check(err)
		require(hash(source) == row.SourceSHA, "source bytes differ")
		text := string(source)
		bodyParts := strings.Split(text, "`")
		require(len(bodyParts) == 3, "body delimiters")
		require(hash([]byte(bodyParts[1])) == row.PartsSHA, "body parts differ")
		partKey := [4]int{row.Family, row.Parameter, row.Parity, row.Form}
		if previous, ok := parts[partKey]; ok {
			require(previous == row.PartsSHA, "opposing goals or prose changed body")
		} else {
			parts[partKey] = row.PartsSHA
		}
		lines := strings.Split(text, "\n")
		stripped := make([]string, 0, len(lines))
		outputs, conditions, choices := 0, 0, 0
		for _, line := range lines {
			if strings.HasPrefix(line, " choice ") {
				choices++
				line = strings.SplitN(line, " intent ", 2)[0]
			}
			if strings.HasPrefix(line, " case ") {
				outputs++
			}
			if strings.HasPrefix(line, " condition_case ") {
				conditions++
			}
			stripped = append(stripped, line)
		}
		require(outputs == 8 && conditions == 3 && choices == 2, "declared shape")
		proseKey := [5]int{row.Family, row.Parameter, row.Goal, row.Parity, row.Form}
		strippedText := strings.Join(stripped, "\n")
		if previous, ok := withoutIntents[proseKey]; ok {
			require(previous == strippedText, "language or wording changed non-intent source")
		} else {
			withoutIntents[proseKey] = strippedText
		}
	}
	require(len(contracts) == 96 && len(coverage) == 1536 && counts["train"] == 768 && counts["validation"] == 384 && counts["evaluation"] == 384, "cohort coverage")
	file, err := os.OpenFile(filepath.Join(root, "sources.tar.gz"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	check(err)
	z := gzip.NewWriter(file)
	archive := tar.NewWriter(z)
	for _, row := range rows {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Source)))
		check(err)
		check(archive.WriteHeader(&tar.Header{Name: row.Source, Mode: 0644, Size: int64(len(data)), ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR}))
		_, err = archive.Write(data)
		check(err)
	}
	check(archive.Close())
	check(z.Close())
	check(file.Close())
	names := []string{"PROTOCOL.md", "prepare.go", "pack.go", "specifications.json", "sources.tar.gz"}
	slices.Sort(names)
	var sums strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, name))
		check(err)
		fmt.Fprintf(&sums, "%s  %s\n", hash(data), name)
	}
	check(os.WriteFile(filepath.Join(root, "SHA256SUMS"), []byte(sums.String()), 0600))
	fmt.Println("Verified1536 source hashes,96goal contracts,all crossed factor combinations,family splits,prose-only changes and identical parts for opposing goals. Packaged source bytes without user/group metadata. No compiler/model/candidate/native calls.")
}
func main() { run() }
