// Package strictjson rejects duplicate fields, malformed UTF-8, unknown fields
// and trailing values before typed decoding. Callers must bound input bytes.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

func Decode(data []byte, output any) error {
	if !utf8.Valid(data) {
		return errors.New("JSON must be valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := value(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("JSON contains trailing data")
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(output)
}

func value(decoder *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("JSON nesting exceeds 32")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, isContainer := token.(json.Delim)
	if !isContainer {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("invalid or duplicate JSON field %q", name)
			}
			seen[name] = true
			if err := value(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := value(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON closing delimiter")
	}
	_, err = decoder.Token()
	return err
}
