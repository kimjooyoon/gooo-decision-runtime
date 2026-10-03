// Package orderprepared reuses immutable candidates for the whole-body judge.
// Source binding remains the compiler caller's responsibility.
package orderprepared

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/orderfacts"
	"github.com/kimjooyoon/gooo-decision-runtime/orderjudge"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

// Runtime owns one snapshot of final model weights and their identities. Copying
// this value shares only privately owned immutable data. The zero value selects
// deterministic fallback order. Callers must not mutate a model during capture.
type Runtime struct {
	model                   *orderjudge.Model
	metadataSHA, weightsSHA string
}

// Identity reports the captured canonical artifact without serializing again.
// Its strings are values; it exposes no mutable weights or runtime state.
type Identity struct {
	MetadataSHA256 string `json:"metadata_sha256,omitempty"`
	WeightsSHA256  string `json:"weights_sha256,omitempty"`
	TensorBytes    int    `json:"tensor_bytes"`
}

func (runtime Runtime) Identity() Identity {
	if runtime.model == nil {
		return Identity{}
	}
	return Identity{runtime.metadataSHA, runtime.weightsSHA, orderjudge.ParameterCount * 4}
}

func NewRuntime(model *orderjudge.Model) (Runtime, error) {
	if model == nil {
		return Runtime{}, nil
	}
	owned := *model
	metadata, weights, err := owned.Marshal()
	if err != nil {
		return Runtime{}, err
	}
	return Runtime{&owned, digest(metadata), digest(weights)}, nil
}

type choice struct {
	id, kind, intentSHA string
	labels              [2]string
}

// Prepared owns exactly eight checked program objects and fixed feature arrays.
// It retains no cases, predictions, selected answers, contexts or mutable maps.
// Every Search owns its results. Do not overwrite a Prepared during use.
type Prepared struct {
	runtime     Runtime
	planSHA     string
	intentSHA   string
	choices     [3]choice
	fallback    uint8
	descriptors [8]orderfacts.Signature
	encoded     [8]string
	programs    [8]*bodyplan.Program
	programSHA  [8]string
	intent      [orderjudge.IntentDim]float32
	candidates  [8][orderjudge.SourceDim]float32
	ranking     [8]uint8
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("context deadline required")
	}
	return ctx.Err()
}

func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

// Prepare validates every composed candidate once. Input may change after this
// method returns, but must not be mutated concurrently with preparation.
func (runtime Runtime) Prepare(ctx context.Context, plan pathplan.Plan) (*Prepared, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if len(plan.Decisions) != 3 {
		return nil, errors.New("three structural choices required")
	}
	root, operands := 0, 0
	for _, c := range plan.Decisions {
		switch c.Kind {
		case pathplan.RootOrder:
			root++
		case pathplan.OperandOrder:
			operands++
		default:
			return nil, errors.New("one root and two operand choices required")
		}
	}
	if root != 1 || operands != 2 {
		return nil, errors.New("one root and two operand choices required")
	}
	base, err := pathplan.Prepare(plan)
	if err != nil {
		return nil, err
	}
	p := &Prepared{runtime: runtime, planSHA: base.PlanSHA256()}
	var intents [3]string
	for i, c := range plan.Decisions {
		p.choices[i] = choice{c.ID, c.Kind, digest([]byte(c.Intent)), [2]string{c.Options[0].Label, c.Options[1].Label}}
		intents[i] = c.Intent
		if c.Fallback == c.Options[1].Label {
			p.fallback |= 1 << i
		}
	}
	intent := strings.Join(intents[:], "\n")
	p.intentSHA = digest([]byte(intent))
	if runtime.model != nil {
		if err := orderjudge.IntentFeatures(intent, &p.intent); err != nil {
			return nil, err
		}
	}
	for mask := range 8 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var expressions [128]bodyplan.Expr
		observed := plan.Base
		observed.Expressions = expressions[:copy(expressions[:], plan.Base.Expressions)]
		order := observed.Root
		for bit, c := range plan.Decisions {
			option := c.Options[(mask>>bit)&1]
			if c.Kind == pathplan.RootOrder {
				order = option.Order
			} else if option.Reverse {
				e := &observed.Expressions[c.Target]
				e.Left, e.Right = e.Right, e.Left
			}
		}
		program, err := base.Compile(p.selected(uint8(mask)))
		if err != nil {
			return nil, fmt.Errorf("composed candidate %d: %w", mask, err)
		}
		if !orderfacts.Encode(observed, order, &p.descriptors[mask]) {
			return nil, fmt.Errorf("candidate %d outside two-update profile", mask)
		}
		if runtime.model != nil {
			if err := orderjudge.SourceFeatures(p.descriptors[mask], &p.candidates[mask]); err != nil {
				return nil, err
			}
		}
		p.programs[mask], p.programSHA[mask] = program, digest([]byte(program.GoooSource()))
		p.encoded[mask], p.ranking[mask] = hex.EncodeToString(p.descriptors[mask][:]), uint8(mask)
	}
	for i := 1; i < 8; i++ {
		for j := i; j > 0 && bits.OnesCount8(p.ranking[j]^p.fallback) < bits.OnesCount8(p.ranking[j-1]^p.fallback); j-- {
			p.ranking[j], p.ranking[j-1] = p.ranking[j-1], p.ranking[j]
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Prepared) PlanSHA256() string {
	if p == nil {
		return ""
	}
	return p.planSHA
}

func (p *Prepared) selected(mask uint8) map[string]string {
	selected := make(map[string]string, 3)
	for bit, c := range p.choices {
		selected[c.id] = c.labels[(mask>>bit)&1]
	}
	return selected
}
