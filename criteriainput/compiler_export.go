package criteriainput

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const MaxCompilerExportBytes = 1 << 20

// FromCompilerExport projects only the explicit criteria_flow source fields.
// expanded_plan and the older model context are deliberately outside Input.
// It checks carrier/digest consistency; source provenance remains compiler-owned.
func FromCompilerExport(raw []byte) (*Prepared, error) {
	if len(raw) == 0 || len(raw) > MaxCompilerExportBytes {
		return nil, errors.New("bounded compiler source export required")
	}
	var envelope struct {
		Schema   string `json:"schema"`
		Source   string `json:"original_source_sha256"`
		Contract string `json:"contract_sha256"`
		Activity string `json:"activity_id"`
		Flow     struct {
			Schema          string          `json:"schema"`
			Status          string          `json:"status"`
			Candidate       json.RawMessage `json:"candidate_plan"`
			CandidateSHA256 string          `json:"candidate_plan_sha256"`
			Predicates      []struct {
				ID          string          `json:"id"`
				Activity    string          `json:"activity_id"`
				Inputs      []InputRole     `json:"inputs"`
				Nodes       json.RawMessage `json:"nodes"`
				GraphSHA256 string          `json:"graph_sha256"`
				ValueFlow   struct {
					Status   string `json:"status"`
					Activity string `json:"activity_id"`
				} `json:"value_flow"`
			} `json:"predicates"`
		} `json:"criteria_flow"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&envelope); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("compiler export contains trailing data")
	}
	flow := envelope.Flow
	if envelope.Schema != "gooo/scalar-criteria-input-export/v1" || flow.Schema != "gooo/criteria-source-flow/v1" || flow.Status != "RESOLVED" || len(flow.Predicates) == 0 || len(flow.Predicates) > MaxPredicates {
		return nil, errors.New("resolved complete scalar source flow required")
	}
	input := Input{Schema: Schema, SourceSHA256: envelope.Source, ContractSHA256: envelope.Contract,
		ActivityID: envelope.Activity, CandidatePlanSHA256: flow.CandidateSHA256}
	if err := decodeCanonicalCarrier(flow.Candidate, &input.Candidate); err != nil {
		return nil, err
	}
	for _, source := range flow.Predicates {
		if source.ValueFlow.Status != "RESOLVED" || source.Activity != source.ValueFlow.Activity || len(source.Inputs) != 2 {
			return nil, errors.New("resolved predicate flow and two source roles required")
		}
		predicate := Predicate{ID: source.ID, ActivityID: source.Activity, GraphSHA256: source.GraphSHA256}
		copy(predicate.Inputs[:], source.Inputs)
		if err := decodeCanonicalCarrier(source.Nodes, &predicate.Nodes); err != nil {
			return nil, err
		}
		input.Predicates = append(input.Predicates, predicate)
	}
	return Prepare(input)
}

// Canonical comparison also catches duplicate keys and oversized fixed arrays
// that encoding/json alone can otherwise silently normalize or truncate.
func decodeCanonicalCarrier(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	canonical, err := json.Marshal(value)
	var compact bytes.Buffer
	if err != nil || json.Compact(&compact, raw) != nil || !bytes.Equal(canonical, compact.Bytes()) {
		return errors.New("complete canonical source carrier required")
	}
	return nil
}
