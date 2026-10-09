#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
out="$here/../requirements.lock"
torch_version="${TORCH_VERSION:-2.14.0}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

uv pip compile "$here/requirements.in" \
  --override "$here/overrides.txt" \
  --universal --python-version 3.11 \
  --generate-hashes --no-annotate --no-header \
  -o "$work/full.lock"

awk '
  /^[^ ]/ { skip = ($0 ~ /^(nvidia-|cuda-|triton|torch)/) }
  !skip { print }
' "$work/full.lock" > "$work/base.lock"

curl -fsS https://download.pytorch.org/whl/cpu/torch/ > "$work/index.html"
hashes="$(grep -oE "torch-${torch_version//./\\.}%2Bcpu-cp3(11|12|13|14)-cp3[0-9]+-manylinux_2_28_(x86_64|aarch64)\.whl#sha256=[0-9a-f]{64}" "$work/index.html" | sed -E 's/.*sha256=//' | sort -u)"
if [ -z "$hashes" ]; then
  echo "no CPU wheels found for torch ${torch_version}" >&2
  exit 1
fi

{
  echo "--extra-index-url https://download.pytorch.org/whl/cpu"
  cat "$work/base.lock"
  printf "torch==%s+cpu ; sys_platform == 'linux' \\\\\n" "$torch_version"
  count="$(printf '%s\n' "$hashes" | wc -l)"
  n=0
  while IFS= read -r h; do
    n=$((n + 1))
    if [ "$n" -lt "$count" ]; then
      printf "    --hash=sha256:%s \\\\\n" "$h"
    else
      printf "    --hash=sha256:%s\n" "$h"
    fi
  done <<< "$hashes"
} > "$out"

echo "wrote $out ($(grep -c '^[a-zA-Z0-9_.-]*==' "$out") packages)"
