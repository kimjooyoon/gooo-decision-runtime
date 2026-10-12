#!/bin/bash
set -euo pipefail
task_study=$(cd "$(dirname "$0")" && pwd)
task_root=$(mktemp -d "${TMPDIR:-/tmp}/gooo-branch-owned-audit.XXXXXX")
trap 'rm -rf "$task_root"' EXIT
cp -R "$task_study/prefit" "$task_root/prefit"
cp -R "$task_study/tool" "$task_root/tool"
(
 cd "$task_root/prefit"
 if command -v sha256sum >/dev/null; then sha256sum -c SHA256SUMS; else shasum -a 256 -c SHA256SUMS; fi
 tar -xzf sources.tar.gz
)
(
 cd "$task_root/tool"
 GOWORK=off GOTOOLCHAIN=go1.27.2 go test -race ./...
)
task_base=https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/resolve/8d5c1c2310017ec5d147b77b891bce894aaa03e4/research/branch-owned-learning-20261012/result
mkdir "$task_root/v3-prose-neutral" "$task_root/v4-branch-owned"
for task_file in corpus.json.gz v3-prose-neutral/observations.json v4-branch-owned/observations.json; do
 curl --fail --silent --show-error --location --max-time 60 "$task_base/$task_file" --output "$task_root/$task_file"
done
cat > "$task_root/RETAINED_SHA256SUMS" <<'SUMS'
8daf73f5cdd96411fe5ef8d14b852a5b8b2732267df31c8bad400445bc89847c  corpus.json.gz
eccbd50ab41a2dd668dd9bf4c6953e7d487c5921e80554b79f71ea1327f9f01b  v3-prose-neutral/observations.json
466f49569d01fa79c0fb6749ad99781cf937549b0258bd8820c38f8077721671  v4-branch-owned/observations.json
SUMS
(
 cd "$task_root"
 if command -v sha256sum >/dev/null; then sha256sum -c RETAINED_SHA256SUMS; else shasum -a 256 -c RETAINED_SHA256SUMS; fi
)
(
 cd "$task_root/tool"
 GOWORK=off GOTOOLCHAIN=go1.27.2 go run . audit "$task_root"
)
cmp "$task_root/audit.json" "$task_study/audit.json"
cmp "$task_root/paired-evaluation.json" "$task_study/paired-evaluation.json"
task_diagnosis="$task_study/../branch-owned-choice-diagnosis-20261012"
GOWORK=off GOTOOLCHAIN=go1.27.2 go run "$task_diagnosis/audit/main.go" "$task_root" > "$task_root/choice-diagnosis.json"
cmp "$task_root/choice-diagnosis.json" "$task_diagnosis/diagnosis.json"
