package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"unsafe"

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
		return fmt.Errorf("usage: criteria-source-input --compiler-export|--packet file.json")
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
	var p *criteriainput.Prepared
	if os.Args[1] == "--compiler-export" {
		p, err = criteriainput.FromCompilerExport(raw)
	} else {
		p, err = criteriainput.Decode(raw)
	}
	if err != nil {
		return err
	}
	result := struct {
		Summary                  criteriainput.Summary `json:"summary"`
		NodeStorageBytes         uintptr               `json:"node_storage_bytes"`
		ReservedNodeArenaBytes   uintptr               `json:"reserved_node_arena_bytes"`
		ReservedLiteralPoolBytes int                   `json:"reserved_literal_pool_bytes"`
		Packet                   json.RawMessage       `json:"packet"`
	}{p.Summary(), unsafe.Sizeof(criteriainput.Node{}), unsafe.Sizeof(criteriainput.Node{}) * criteriainput.MaxNodes,
		criteriainput.MaxLiteralBytes, p.Canonical()}
	return json.NewEncoder(os.Stdout).Encode(result)
}
