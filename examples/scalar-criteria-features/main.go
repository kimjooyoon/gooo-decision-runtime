package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kimjooyoon/gooo-decision-runtime/criteriainput"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 3 || (os.Args[1] != "--compiler-export" && os.Args[1] != "--packet") {
		return fmt.Errorf("usage: scalar-criteria-features --compiler-export|--packet file.json")
	}
	file, err := os.Open(os.Args[2])
	if err != nil {
		return err
	}
	defer file.Close()
	limit := criteriainput.MaxCompilerExportBytes
	if os.Args[1] == "--packet" {
		limit = criteriainput.MaxBytes
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return err
	}
	var prepared *criteriainput.Prepared
	if os.Args[1] == "--compiler-export" {
		prepared, err = criteriainput.FromCompilerExport(raw)
	} else {
		prepared, err = criteriainput.Decode(raw)
	}
	if err != nil {
		return err
	}
	var features criteriainput.Features
	if err := prepared.FeaturesInto(&features); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Summary  criteriainput.Summary  `json:"summary"`
		Packet   json.RawMessage        `json:"packet"`
		Features criteriainput.Features `json:"features"`
	}{prepared.Summary(), prepared.Canonical(), features})
}
