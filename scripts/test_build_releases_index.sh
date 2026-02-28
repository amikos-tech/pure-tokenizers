#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${ROOT_DIR}/scripts/build_releases_index.sh"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for tests." >&2
  exit 1
fi

if [[ ! -x "$SCRIPT" ]]; then
  echo "Builder script is missing or not executable: ${SCRIPT}" >&2
  exit 1
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"
  if [[ "$actual" != "$expected" ]]; then
    echo "Assertion failed: ${message}" >&2
    echo "  expected: ${expected}" >&2
    echo "  actual:   ${actual}" >&2
    exit 1
  fi
}

assert_fails() {
  local message="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "Assertion failed: ${message}" >&2
    exit 1
  fi
}

run_builder() {
  local existing="$1"
  local output="$2"
  "${SCRIPT}" \
    --existing "$existing" \
    --output "$output" \
    --project pure-tokenizers \
    --version v0.1.4 \
    --date 2026-03-01T00:00:00Z \
    --max 3
}

# Case 1: missing file -> initialize from empty
missing_existing="${work_dir}/missing.json"
out_missing="${work_dir}/out-missing.json"
run_builder "$missing_existing" "$out_missing"
assert_eq "$(jq -r '.releases | length' "$out_missing")" "1" "missing input should create one entry"
assert_eq "$(jq -r '.releases[0].version' "$out_missing")" "v0.1.4" "missing input should use requested version"
assert_eq "$(jq -r '.releases[0].prefix' "$out_missing")" "pure-tokenizers/v0.1.4/" "missing input should use normalized prefix"
assert_eq "$(jq -r '.releases[0].checksums_url' "$out_missing")" "pure-tokenizers/v0.1.4/SHA256SUMS" "missing input should use expected checksums URL"

# Case 2: malformed JSON -> fallback to empty
malformed_existing="${work_dir}/malformed.json"
out_malformed="${work_dir}/out-malformed.json"
echo 'not-json' >"$malformed_existing"
run_builder "$malformed_existing" "$out_malformed"
assert_eq "$(jq -r '.releases | length' "$out_malformed")" "1" "malformed input should fallback to one entry"

# Case 3: wrong schema -> fallback to empty
wrong_schema_existing="${work_dir}/wrong-schema.json"
out_wrong_schema="${work_dir}/out-wrong-schema.json"
echo '{"releases":"invalid"}' >"$wrong_schema_existing"
run_builder "$wrong_schema_existing" "$out_wrong_schema"
assert_eq "$(jq -r '.releases | length' "$out_wrong_schema")" "1" "wrong schema should fallback to one entry"

# Case 4: mixed array entries -> keep only valid objects and add new entry
mixed_existing="${work_dir}/mixed.json"
out_mixed="${work_dir}/out-mixed.json"
cat >"$mixed_existing" <<'EOF'
{
  "releases": [
    "bad",
    {
      "version": "v0.1.3",
      "date": "2026-02-28T00:00:00Z",
      "prefix": "pure-tokenizers/v0.1.3/",
      "checksums_url": "pure-tokenizers/v0.1.3/SHA256SUMS"
    },
    {
      "version": "",
      "date": "",
      "prefix": "",
      "checksums_url": ""
    }
  ]
}
EOF
run_builder "$mixed_existing" "$out_mixed"
assert_eq "$(jq -r '.releases | length' "$out_mixed")" "2" "mixed input should preserve only valid entries plus new entry"

# Case 5: duplicate version in existing -> prefer new entry
duplicate_existing="${work_dir}/duplicate.json"
out_duplicate="${work_dir}/out-duplicate.json"
cat >"$duplicate_existing" <<'EOF'
{
  "releases": [
    {
      "version": "v0.1.4",
      "date": "2026-02-01T00:00:00Z",
      "prefix": "pure-tokenizers/v0.1.4/",
      "checksums_url": "pure-tokenizers/v0.1.4/SHA256SUMS"
    },
    {
      "version": "v0.1.3",
      "date": "2026-01-01T00:00:00Z",
      "prefix": "pure-tokenizers/v0.1.3/",
      "checksums_url": "pure-tokenizers/v0.1.3/SHA256SUMS"
    }
  ]
}
EOF
run_builder "$duplicate_existing" "$out_duplicate"
assert_eq "$(jq -r '.releases | length' "$out_duplicate")" "2" "duplicate versions should be de-duplicated"
assert_eq "$(jq -r '.releases[] | select(.version=="v0.1.4") | .date' "$out_duplicate")" "2026-03-01T00:00:00Z" "new release entry should override existing duplicate"

# Case 6: max bound should be enforced
bounded_existing="${work_dir}/bounded.json"
out_bounded="${work_dir}/out-bounded.json"
cat >"$bounded_existing" <<'EOF'
{
  "releases": [
    {
      "version": "v0.1.3",
      "date": "2026-02-28T00:00:00Z",
      "prefix": "pure-tokenizers/v0.1.3/",
      "checksums_url": "pure-tokenizers/v0.1.3/SHA256SUMS"
    },
    {
      "version": "v0.1.2",
      "date": "2026-02-27T00:00:00Z",
      "prefix": "pure-tokenizers/v0.1.2/",
      "checksums_url": "pure-tokenizers/v0.1.2/SHA256SUMS"
    },
    {
      "version": "v0.1.1",
      "date": "2026-02-26T00:00:00Z",
      "prefix": "pure-tokenizers/v0.1.1/",
      "checksums_url": "pure-tokenizers/v0.1.1/SHA256SUMS"
    }
  ]
}
EOF
run_builder "$bounded_existing" "$out_bounded"
assert_eq "$(jq -r '.releases | length' "$out_bounded")" "3" "max bound should cap index size"
assert_eq "$(jq -r '.releases[0].version' "$out_bounded")" "v0.1.4" "newest release should be first"
assert_eq "$(jq -r '.releases[1].version' "$out_bounded")" "v0.1.3" "second newest release should follow"
assert_eq "$(jq -r '.releases[2].version' "$out_bounded")" "v0.1.2" "oldest retained release should be last"

# Case 7: invalid --date should fail fast
invalid_date_existing="${work_dir}/invalid-date.json"
out_invalid_date="${work_dir}/out-invalid-date.json"
echo '{"releases":[]}' >"$invalid_date_existing"
assert_fails \
  "invalid --date should fail." \
  "${SCRIPT}" \
  --existing "$invalid_date_existing" \
  --output "$out_invalid_date" \
  --project pure-tokenizers \
  --version v0.1.4 \
  --date invalid-date \
  --max 3

# Case 8: argument validation should fail on bad flags and values
assert_fails "missing required args should fail." "${SCRIPT}" --output "$out_invalid_date"
assert_fails "--max must reject zero." "${SCRIPT}" --output "$out_invalid_date" --project pure-tokenizers --version v0.1.4 --date 2026-03-01T00:00:00Z --max 0
assert_fails "--max must reject non-integers." "${SCRIPT}" --output "$out_invalid_date" --project pure-tokenizers --version v0.1.4 --date 2026-03-01T00:00:00Z --max abc
assert_fails "unknown flags should fail." "${SCRIPT}" --output "$out_invalid_date" --project pure-tokenizers --version v0.1.4 --date 2026-03-01T00:00:00Z --unknown value
assert_fails "trailing --max without value should fail." "${SCRIPT}" --output "$out_invalid_date" --project pure-tokenizers --version v0.1.4 --date 2026-03-01T00:00:00Z --max

echo "All release index tests passed."
