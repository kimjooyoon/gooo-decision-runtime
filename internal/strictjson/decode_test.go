package strictjson

import "testing"

func TestStrictDecode(t *testing.T) {
	for _, input := range [][]byte{
		[]byte(`{"name":"a","name":"b"}`),
		[]byte(`{"extra":1}`),
		[]byte(`{"name":"a"} {}`),
		{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'},
		[]byte(`{"name":`),
	} {
		var target struct {
			Name string `json:"name"`
		}
		if err := Decode(input, &target); err == nil {
			t.Fatalf("accepted invalid input %q", input)
		}
	}
	var target struct {
		Name string `json:"name"`
	}
	if err := Decode([]byte(`{"name":"valid"}`), &target); err != nil || target.Name != "valid" {
		t.Fatalf("valid input: %+v %v", target, err)
	}
}
