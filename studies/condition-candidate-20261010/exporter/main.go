package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

type record struct {
	ID       string            `json:"id"`
	Split    string            `json:"split"`
	Source   string            `json:"gooo_source"`
	SHA      string            `json:"source_sha256"`
	Document pathplan.Document `json:"document"`
}

func main() {
	if len(os.Args) != 2 {
		panic("output file required")
	}
	var records []record
	for family := range 4 {
		for polarity := range 2 {
			for wording := range 3 {
				split := "train"
				if wording == 2 {
					split = "wording"
				}
				if family >= 2 {
					split = "structure"
					if wording == 2 {
						split = "both"
					}
				}
				body := "if input < 0 { return 0 - input } else { return input }"
				if family%2 == 1 {
					body = "if 0 < input { return input } else { return 0 - input }"
				}
				if family >= 2 {
					body = "let value = input\n" + strings.ReplaceAll(body, "input", "value")
				}
				comparison := [2][3]string{{"Compare whether the input is negative.", "입력이 음수인지 비교한다.", "Is the incoming integer below zero? 음수 조건을 선택한다."}, {"Compare whether the input is positive.", "입력이 양수인지 비교한다.", "Is the incoming integer above zero? 양수 조건을 선택한다."}}[polarity][wording]
				branch := [2][3]string{{"Use the true branch for negative inputs.", "음수일 때 참 분기를 사용한다.", "Send numbers below zero through the true arm. 음수 분기."}, {"Use the true branch for positive inputs.", "양수일 때 참 분기를 사용한다.", "Send numbers above zero through the true arm. 양수 분기."}}[polarity][wording]
				var s strings.Builder
				fmt.Fprintf(&s, "package conditionstudy\nnamespace conditionstudy\nentity Integer id \"conditionstudy://integer\"\nactivity Choose(Integer) -> Integer computes %s assembling {\n", strconv.Quote(body))
				fmt.Fprintf(&s, " choice \"comparison\" operand_order at \"0\" intent %s\n choice \"branches\" branch_layout at \"0\" intent %s\n", strconv.Quote(comparison), strconv.Quote(branch))
				for _, x := range []int64{-9, -2, 0, 2, 9, -9007199254740995, 9007199254740995} {
					expected := x
					if expected < 0 {
						expected = -expected
					}
					fmt.Fprintf(&s, " case \"%d\" -> \"%d\"\n", x, expected)
				}
				for _, x := range []int64{-9007199254740995, 0, 9007199254740995} {
					expected := x < 0
					if polarity == 1 {
						expected = x > 0
					}
					fmt.Fprintf(&s, " condition_case \"comparison\" input \"%d\" -> \"%t\"\n", x, expected)
				}
				s.WriteString(" attempts \"4\"\n}\n")
				id := fmt.Sprintf("family%d-polarity%d-wording%d", family, polarity, wording)
				source := s.String()
				doc, err := bodycodegen.DecodeSourcePathDocument(context.Background(), id+".gooo", []byte(source), "Choose", nil)
				if err != nil {
					panic(fmt.Errorf("%s: %w", id, err))
				}
				hash := sha256.Sum256([]byte(source))
				records = append(records, record{id, split, source, hex.EncodeToString(hash[:]), doc})
			}
		}
	}
	file, err := os.OpenFile(os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic(err)
	}
	if err := json.NewEncoder(file).Encode(records); err != nil {
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
	fmt.Printf("exported %d source-derived Gooo documents\n", len(records))
}
