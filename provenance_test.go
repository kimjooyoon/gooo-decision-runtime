package decision_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestProductionSourceMatchesPinnedOrigin(t *testing.T) {
	data, err := os.ReadFile("source-provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Schema           string `json:"schema"`
		Module           string `json:"module"`
		OriginRepository string `json:"origin_repository"`
		OriginRevision   string `json:"origin_revision"`
		Files            []struct {
			Origin string `json:"origin_path"`
			Copied string `json:"copied_path"`
			SHA256 string `json:"sha256"`
			Bytes  int    `json:"bytes"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != "gooo/decision-runtime-source-provenance/v1" ||
		manifest.Module != "github.com/kimjooyoon/gooo-decision-runtime" ||
		manifest.OriginRepository != "github.com/kimjooyoon/gooo-neural-decision-experiments" ||
		manifest.OriginRevision != "e18908b88eecaacbb0b80df4feb8cc7b9b4380bb" {
		t.Fatal("source manifest identity changed")
	}
	want := map[string]struct {
		origin string
		sha256 string
		bytes  int
	}{
		"model.go":  {"internal/decision/model.go", "5444e9a649a1cd2bd84f6b157b22e7fc912bc912ce046f5bda1f63cb6c6c5a02", 22717},
		"bridge.go": {"internal/decision/bridge.go", "08227b64151ee8296c743e409d5a7dbb61f65d4d841d4dc0e0ab3a6119607236", 3745},
		"ir.go":     {"internal/decision/ir.go", "14b259d00a581fe8924f973befa0149052f1f8dacb1f2f5fa03eef1eea24312d", 3625},
		"LICENSE":   {"LICENSE", "3ad2cd8fe84a937a0005a2934e377432f2f86fe10ff86c4242cc48f49ebf4947", 1074},
	}
	if len(manifest.Files) != len(want) {
		t.Fatal("source manifest count changed")
	}
	for _, entry := range manifest.Files {
		pinned, ok := want[entry.Copied]
		if !ok || entry.Origin != pinned.origin || entry.SHA256 != pinned.sha256 || entry.Bytes != pinned.bytes {
			t.Fatalf("unexpected or repeated source entry %q", entry.Copied)
		}
		delete(want, entry.Copied)
		copied, err := os.ReadFile(entry.Copied)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(copied)
		if len(copied) != entry.Bytes || hex.EncodeToString(digest[:]) != entry.SHA256 {
			t.Fatalf("copied source %s no longer matches recorded origin bytes", entry.Copied)
		}
	}
}
