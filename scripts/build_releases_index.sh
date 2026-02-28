#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  build_releases_index.sh --output <path> --project <name> --version <tag> --date <rfc3339> [--existing <path>] [--max <n>]

Builds a bounded releases.json index for R2 release metadata.
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Required command not found: ${cmd}" >&2
    exit 1
  fi
}

require_cmd jq

existing_path=""
output_path=""
project=""
version=""
release_date=""
max_releases=100

while [[ $# -gt 0 ]]; do
  case "$1" in
    --existing)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for $1." >&2
        usage
        exit 1
      fi
      existing_path="${2:-}"
      shift 2
      ;;
    --output)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for $1." >&2
        usage
        exit 1
      fi
      output_path="${2:-}"
      shift 2
      ;;
    --project)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for $1." >&2
        usage
        exit 1
      fi
      project="${2:-}"
      shift 2
      ;;
    --version)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for $1." >&2
        usage
        exit 1
      fi
      version="${2:-}"
      shift 2
      ;;
    --date)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for $1." >&2
        usage
        exit 1
      fi
      release_date="${2:-}"
      shift 2
      ;;
    --max)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for $1." >&2
        usage
        exit 1
      fi
      max_releases="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$output_path" || -z "$project" || -z "$version" || -z "$release_date" ]]; then
  echo "Missing required arguments." >&2
  usage
  exit 1
fi

if ! [[ "$max_releases" =~ ^[0-9]+$ ]] || [[ "$max_releases" -le 0 ]]; then
  echo "--max must be a positive integer." >&2
  exit 1
fi

if ! jq -en --arg date "$release_date" '$date | fromdateiso8601' >/dev/null 2>&1; then
  echo "--date must be a valid UTC timestamp (format: YYYY-MM-DDTHH:MM:SSZ)." >&2
  exit 1
fi

mkdir -p "$(dirname "$output_path")"

tmp_existing="$(mktemp)"
tmp_output="$(mktemp)"
trap 'rm -f "$tmp_existing" "$tmp_output"' EXIT

if [[ -n "$existing_path" ]] && [[ -f "$existing_path" ]]; then
  if jq -e '.releases | type == "array"' "$existing_path" >/dev/null 2>&1; then
    cp "$existing_path" "$tmp_existing"
  else
    echo "Warning: existing releases index at ${existing_path} is invalid; starting from empty index." >&2
    echo '{"releases":[]}' >"$tmp_existing"
  fi
else
  echo '{"releases":[]}' >"$tmp_existing"
fi

new_entry="$(jq -n \
  --arg version "$version" \
  --arg date "$release_date" \
  --arg prefix "${project}/${version}/" \
  --arg checksums_url "${project}/${version}/SHA256SUMS" \
  '{version: $version, date: $date, prefix: $prefix, checksums_url: $checksums_url}')"

jq \
  --argjson entry "$new_entry" \
  --argjson max "$max_releases" '
def normalize:
  if type != "object" then
    empty
  else
    {
      version: (.version // empty),
      date: (.date // empty),
      prefix: (.prefix // empty),
      checksums_url: (.checksums_url // empty)
    }
    | select(
        (.version | type == "string" and length > 0) and
        (.date | type == "string" and length > 0) and
        (.prefix | type == "string" and length > 0) and
        (.checksums_url | type == "string" and length > 0)
      )
  end;
def dedupe_first_by_version:
  reduce .[] as $item (
    [];
    if any(.[]; .version == $item.version) then . else . + [$item] end
  );
{
  releases:
    (
      ([ $entry ] + ((.releases // []) | map(normalize)))
      | dedupe_first_by_version
      | sort_by(.date)
      | reverse
      | .[:$max]
    )
}
' "$tmp_existing" >"$tmp_output"

jq -e '.releases | type == "array"' "$tmp_output" >/dev/null
mv "$tmp_output" "$output_path"
