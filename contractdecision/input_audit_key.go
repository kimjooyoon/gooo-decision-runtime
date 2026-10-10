package contractdecision

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func auditCaseVersion(cases CaseSource) string {
	if reader, ok := cases.(interface{ CaseFeatureVersion() string }); ok {
		return reader.CaseFeatureVersion()
	}
	return decision.DeclaredCaseFeatureVersion
}

func auditInputDigest(ctx context.Context, sample Sample) ([32]byte, error) {
	var digest [32]byte
	if err := ctx.Err(); err != nil {
		return digest, err
	}
	if err := validate(sample.Inputs, sample.Cases, sample.Masks); err != nil {
		return digest, err
	}
	version := auditCaseVersion(sample.Cases)
	if version != decision.DeclaredCaseFeatureVersion && version != decision.SourceLiteralCaseFeatureVersion {
		return digest, errors.New("unsupported input-audit case feature version")
	}
	if sample.Acceptable == 0 || sample.Acceptable & ^allCandidates(len(sample.Masks)) != 0 {
		return digest, errors.New("acceptable set must name supplied complete candidates")
	}
	count := sample.Cases.CaseCount()
	if count < 1 || count > MaxCases {
		return digest, errors.New("case count changed outside audit bounds")
	}
	h := sha256.New()
	h.Write([]byte("gooo/decision-input-audit/v1\x00" + decision.RelationalFlowFeatureVersion + "\x00" + version + "\x00"))
	var header [12]byte
	binary.LittleEndian.PutUint32(header[0:4], uint32(len(sample.Inputs)))
	binary.LittleEndian.PutUint32(header[4:8], uint32(count))
	binary.LittleEndian.PutUint32(header[8:12], uint32(len(sample.Masks)))
	h.Write(header[:])
	var cells [FeatureDim * 4]byte
	writeRow := func(values []float32) {
		for i, value := range values {
			binary.LittleEndian.PutUint32(cells[i*4:i*4+4], math.Float32bits(value))
		}
		h.Write(cells[:len(values)*4])
	}
	for _, input := range sample.Inputs {
		writeRow(input[:])
	}
	for i := range count {
		if err := ctx.Err(); err != nil {
			return digest, err
		}
		row, err := sample.Cases.CaseFeatures(i)
		if err != nil {
			return digest, err
		}
		if !finite(row[:]) {
			return digest, errors.New("nonfinite audit case")
		}
		writeRow(row[:])
	}
	for _, mask := range sample.Masks {
		binary.LittleEndian.PutUint16(cells[:2], mask)
		h.Write(cells[:2])
	}
	copy(digest[:], h.Sum(nil))
	return digest, nil
}

func sameAuditInput(ctx context.Context, a, b Sample) (bool, error) {
	if len(a.Inputs) != len(b.Inputs) || a.Cases.CaseCount() != b.Cases.CaseCount() ||
		auditCaseVersion(a.Cases) != auditCaseVersion(b.Cases) || !slices.Equal(a.Masks, b.Masks) {
		return false, nil
	}
	for i := range a.Inputs {
		if !sameAuditCells(a.Inputs[i][:], b.Inputs[i][:]) {
			return false, nil
		}
	}
	for i := range a.Cases.CaseCount() {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		x, err := a.Cases.CaseFeatures(i)
		if err != nil {
			return false, err
		}
		y, err := b.Cases.CaseFeatures(i)
		if err != nil {
			return false, err
		}
		if !sameAuditCells(x[:], y[:]) {
			return false, nil
		}
	}
	return true, nil
}

func sameAuditCells(a, b []float32) bool {
	for i := range a {
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) {
			return false
		}
	}
	return true
}
