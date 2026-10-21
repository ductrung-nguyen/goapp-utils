#!/bin/sh
set -eu

semantic_release=${1:?usage: scripts/release.sh /path/to/semantic-release}

if ! output=$("$semantic_release" --plugin-resolver github --dry --allow-no-changes 2>&1); then
  printf '%s\n' "$output"
  exit 1
fi
printf '%s\n' "$output"

version=$(printf '%s\n' "$output" | awk -F 'new version: ' '/new version: / { print $2; exit }')
if [ -z "$version" ]; then
  exit 0
fi

scripts/check-go-module-version.sh "$version"

exec "$semantic_release" --plugin-resolver github --allow-no-changes
