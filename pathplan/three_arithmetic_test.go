package pathplan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

func TestThreeInitialAndFeedbackArithmeticIdentity(t *testing.T) {
	for _, arithmetic := range []string{"", jointdecision.SeparateArithmeticVersion} {
		p, ctx := threePrepared(t, false), threeContext(t)
		m := testSharedThreeContract(t, jointdecision.ThreeBagFeatureVersion, arithmetic)
		s, err := p.NewThreeSession(ctx, m, []TestCase{{Input: 3, Expected: 999}}, "")
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.Observe()
		if err != nil || r.Selection.Three.Arithmetic != arithmetic {
			t.Fatal("initial arithmetic receipt", err)
		}
		b, err := json.Marshal(r.Selection.Three)
		if err != nil || strings.Contains(string(b), "arithmetic_version") != (arithmetic != "") {
			t.Fatal("legacy receipt changed or explicit identity omitted", err)
		}
		if _, _, err = s.Advance(ctx, 1); err != nil {
			t.Fatal(err)
		}
		f, err := s.ReconsiderThree(ctx, m, nil)
		if err != nil || f.Three == nil || f.Three.Arithmetic != arithmetic || f.ModelCalls != 1 {
			t.Fatal("feedback arithmetic receipt", err)
		}
	}
}
