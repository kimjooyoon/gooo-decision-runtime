#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo 'usage: fetch.sh <new-output-directory>' >&2
  exit 2
fi
destination=$1
mkdir "$destination"
base='https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/resolve/0aa1dd8c51e730632f0f4bc590b5498799178492/research/factorized-decision-20261012/result'
for item in corpus.json.gz v2-full/observations.json v3-full/observations.json v3-prose-neutral/observations.json; do
  mkdir -p "$destination/$(dirname "$item")"
  curl --fail --location --silent --show-error --connect-timeout 20 --max-time 120 --max-filesize 8388608 \
    "$base/$item" --output "$destination/$item"
done
