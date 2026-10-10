package decision

import (
	"math"
	"math/big"
	"testing"
)

func TestSourceLiteralCasePreservesBytesAndExactRelations(t *testing.T) {
	for _, pair := range [][2]int64{{3, 10}, {3, -4}, {9007199254740993, 9007199254741000}, {math.MinInt64, math.MaxInt64}} {
		var old, row [32]float32
		if err := DeclaredCaseFeaturesInto(pair[0], pair[1], &old); err != nil {
			t.Fatal(err)
		}
		if err := SourceLiteralCaseFeaturesInto(pair[0], pair[1], []int64{7}, &row); err != nil {
			t.Fatal(err)
		}
		for i := 12; i < 28; i++ {
			if row[i] != old[i] {
				t.Fatal("integer byte changed", i)
			}
		}
		for i := 7; i < 10; i++ {
			if row[i] != old[i] {
				t.Fatal("input relation changed", i)
			}
		}
		if row[0] != .125 || row[1] != .125 {
			t.Fatal("presence", row)
		}
	}
	var row [32]float32
	_ = SourceLiteralCaseFeaturesInto(3, 10, []int64{7}, &row)
	if row[3] != .125 || row[10] != .125 || row[11] != .125 || row[28] != 0 || row[31] != 0 {
		t.Fatal("sum/literal relations", row)
	}
	_ = SourceLiteralCaseFeaturesInto(3, -4, []int64{7}, &row)
	if row[28] != .125 {
		t.Fatal("difference relation", row)
	}
	_ = SourceLiteralCaseFeaturesInto(math.MaxInt64, math.MinInt64, []int64{1}, &row)
	if row[11] != 0 {
		t.Fatal("overflow became an exact sum")
	}
	_ = SourceLiteralCaseFeaturesInto(math.MinInt64, math.MinInt64, []int64{-1}, &row)
	if row[31] != 0 {
		t.Fatal("overflow became an exact product")
	}
	var literals [128]int64
	for i := range literals {
		literals[i] = int64(i)
	}
	_ = SourceLiteralCaseFeaturesInto(1, 127, literals[:], &row)
	if row[5] != 1.0/1024 || row[31] != 1.0/1024 {
		t.Fatal("last literal omitted", row)
	}
	if allocations := testing.AllocsPerRun(10, func() { _ = SourceLiteralCaseFeaturesInto(1, 127, literals[:], &row) }); allocations != 0 {
		t.Fatal("projection allocation", allocations)
	}
	before := row
	for _, bad := range [][]int64{{1, 1}, {2, 1}, make([]int64, 129)} {
		if err := SourceLiteralCaseFeaturesInto(1, 2, bad, &row); err == nil || row != before {
			t.Fatal("non-atomic invalid literals", err)
		}
	}
	if SourceLiteralCaseFeaturesInto(1, 2, nil, nil) == nil {
		t.Fatal("nil destination")
	}
}

func TestSourceLiteralArithmeticAgainstExactIntegerReference(t *testing.T) {
	values := []int64{math.MinInt64, math.MinInt64 + 1, -9007199254740995, -1, 0, 1, 2, 9007199254740993, math.MaxInt64 - 1, math.MaxInt64}
	check := func(a, b int64) {
		for operation, fn := range []func(int64, int64) (int64, bool){sourceCaseAdd, sourceCaseSub, sourceCaseMul} {
			var exact big.Int
			left, right := big.NewInt(a), big.NewInt(b)
			switch operation {
			case 0:
				exact.Add(left, right)
			case 1:
				exact.Sub(left, right)
			case 2:
				exact.Mul(left, right)
			}
			got, ok := fn(a, b)
			if ok != exact.IsInt64() || ok && got != exact.Int64() {
				t.Fatal("arithmetic relation", operation, a, b, got, ok, &exact)
			}
		}
	}
	for _, a := range values {
		for _, b := range values {
			check(a, b)
		}
	}
	state := uint64(17)
	next := func() int64 { state ^= state << 13; state ^= state >> 7; state ^= state << 17; return int64(state) }
	for range 4096 {
		check(next(), next())
	}
}
