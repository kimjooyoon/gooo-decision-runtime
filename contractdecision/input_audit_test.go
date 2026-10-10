package contractdecision

import (
	"context"
	"math"
	"reflect"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func auditSample(acceptable uint64) Sample {
	return Sample{Inputs: make([][FeatureDim]float32, 2), Cases: rows{{}},
		Masks: []uint16{0, 1, 2}, Acceptable: acceptable}
}

func TestInputAuditFindsJointConflictWithPairwiseOverlaps(t *testing.T) {
	// Every pair shares a valid candidate, but no one candidate satisfies all.
	samples := []Sample{auditSample(3), auditSample(6), auditSample(5)}
	audit, err := AuditInputs(context.Background(), samples)
	if err != nil || audit.Samples != 3 || len(audit.Groups) != 1 || audit.RepeatedGroups != 1 ||
		audit.ConflictingGroups != 1 || audit.BestFirstPasses != 2 || audit.UnavoidableMisses != 1 {
		t.Fatal(audit, err)
	}
	group := audit.Groups[0]
	if group.CommonAcceptable != 0 || group.BestFirstPasses != 2 || group.UnavoidableMisses != 1 ||
		!reflect.DeepEqual(group.SampleIndices, []int{0, 1, 2}) || len(group.InputSHA256) != 64 {
		t.Fatal(group)
	}
	// Labels do not enter the feature identity; a shared acceptable path removes
	// the input conflict without changing the model input or its digest.
	for i := range samples {
		samples[i].Acceptable |= 1
	}
	shared, err := AuditInputs(context.Background(), samples)
	if err != nil || shared.ConflictingGroups != 0 || shared.BestFirstPasses != 3 ||
		shared.UnavoidableMisses != 0 || shared.Groups[0].InputSHA256 != group.InputSHA256 {
		t.Fatal(shared, err)
	}
	audit.Groups[0].CandidateMasks[0] = 99
	if samples[0].Masks[0] != 0 {
		t.Fatal("returned report aliases the caller's masks")
	}
}

type auditVersionRows struct {
	rows
	version string
}

func (r auditVersionRows) CaseFeatureVersion() string { return r.version }

func TestInputAuditUsesCompleteOrderedBitInputs(t *testing.T) {
	base := auditSample(1)
	base.Cases = make(rows, MaxCases)
	variants := []Sample{base}
	appendVariant := func(edit func(*Sample)) {
		next := Sample{Inputs: append([][FeatureDim]float32(nil), base.Inputs...),
			Cases: append(rows(nil), base.Cases.(rows)...), Masks: append([]uint16(nil), base.Masks...), Acceptable: 2}
		edit(&next)
		variants = append(variants, next)
	}
	appendVariant(func(s *Sample) { s.Inputs[1][FeatureDim-1] = 1 })
	appendVariant(func(s *Sample) { s.Cases.(rows)[MaxCases-1][CaseDim-1] = 1 })
	appendVariant(func(s *Sample) { s.Inputs[0][0] = float32(math.Copysign(0, -1)) })
	appendVariant(func(s *Sample) { s.Masks[0], s.Masks[1] = s.Masks[1], s.Masks[0] })
	appendVariant(func(s *Sample) { s.Cases = auditVersionRows{s.Cases.(rows), decision.SourceLiteralCaseFeatureVersion} })
	appendVariant(func(s *Sample) { s.Cases = s.Cases.(rows)[:MaxCases-1] })
	for i, sample := range variants[1:] {
		same, err := sameAuditInput(context.Background(), base, sample)
		if err != nil || same {
			t.Fatal("exact comparison missed changed input", i, err)
		}
	}
	audit, err := AuditInputs(context.Background(), variants)
	if err != nil || len(audit.Groups) != len(variants) || audit.RepeatedGroups != 0 ||
		audit.ConflictingGroups != 0 || audit.BestFirstPasses != len(variants) {
		t.Fatal(audit, err)
	}
}

func TestInputAuditPreservesLargeIntegerCaseDifferences(t *testing.T) {
	for _, value := range []int64{9007199254740993, -9007199254740995, math.MinInt64} {
		samples := []Sample{auditSample(1), auditSample(2)}
		for i := range samples {
			var row [CaseDim]float32
			if err := decision.DeclaredCaseFeaturesInto(value, value+int64(i), &row); err != nil {
				t.Fatal(err)
			}
			samples[i].Cases = rows{row}
		}
		audit, err := AuditInputs(context.Background(), samples)
		if err != nil || len(audit.Groups) != 2 || audit.UnavoidableMisses != 0 {
			t.Fatal("one-unit goal difference was merged", value, audit, err)
		}
	}
}

func TestInputAuditKeepsAll64CandidateLabels(t *testing.T) {
	sample := Sample{Inputs: make([][FeatureDim]float32, MaxChoices), Cases: rows{{}},
		Masks: make([]uint16, MaxCandidates), Acceptable: uint64(1) << 63}
	for i := range sample.Masks {
		sample.Masks[i] = uint16(i)
	}
	other := sample
	other.Acceptable = math.MaxUint64
	audit, err := AuditInputs(context.Background(), []Sample{sample, other})
	if err != nil || audit.BestFirstPasses != 2 || audit.Groups[0].CommonAcceptable != uint64(1)<<63 {
		t.Fatal(audit, err)
	}
}

func TestInputAuditRejectsInvalidInputsAndCancellation(t *testing.T) {
	ctx := context.Background()
	for _, edit := range []func(*Sample){
		func(s *Sample) { s.Inputs = nil },
		func(s *Sample) { s.Inputs[0][0] = float32(math.NaN()) },
		func(s *Sample) { s.Cases = rows{{float32(math.Inf(1))}} },
		func(s *Sample) { s.Cases = &countedCases{n: 128, fail: 127} },
		func(s *Sample) { s.Cases = make(rows, 129) },
		func(s *Sample) { s.Cases = auditVersionRows{rows{{}}, "unknown"} },
		func(s *Sample) { s.Acceptable = 0 },
		func(s *Sample) { s.Acceptable = 8 },
		func(s *Sample) { s.Masks[1] = s.Masks[0] },
	} {
		bad := auditSample(1)
		edit(&bad)
		if audit, err := AuditInputs(ctx, []Sample{auditSample(1), bad}); err == nil || audit != nil {
			t.Fatal("invalid input returned a partial audit", audit, err)
		}
	}
	for _, samples := range [][]Sample{nil, make([]Sample, 4097)} {
		if audit, err := AuditInputs(ctx, samples); err == nil || audit != nil {
			t.Fatal("sample bound", err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, context := range []context.Context{nil, cancelled} {
		if audit, err := AuditInputs(context, []Sample{auditSample(1)}); err == nil || audit != nil {
			t.Fatal("context bound", err)
		}
	}
}

func TestInputAuditConcurrentCallsAreDeterministic(t *testing.T) {
	samples := []Sample{auditSample(1), auditSample(2), auditSample(3)}
	want, err := AuditInputs(context.Background(), samples)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			got, err := AuditInputs(context.Background(), samples)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Error(got, err)
			}
		})
	}
	wg.Wait()
}
