#!/bin/sh
set -eu

version=${1:?usage: scripts/check-go-module-version.sh version}
major=${version%%.*}
case "$major" in
  ''|*[!0-9]*)
    printf 'Could not determine release major version from %s.\n' "$version" >&2
    exit 1
    ;;
esac

module_path=$(awk '$1 == "module" { gsub(/^"|"$/, "", $2); print $2; exit }' go.mod)
if [ -z "$module_path" ]; then
  printf 'Cannot determine Go module path from go.mod.\n' >&2
  exit 1
fi

if [ "$major" -gt 1 ]; then
  case "$module_path" in
    */v"$major") ;;
    *)
      printf 'Go module %s cannot be released as v%s; update the module path to end in /v%s and update imports first.\n' "$module_path" "$version" "$major" >&2
      exit 1
      ;;
  esac
else
  case "$module_path" in
    */v[0-9]*)
      printf 'Go module %s cannot be released as v%s; remove its /vN suffix before releasing a v0 or v1 module.\n' "$module_path" "$version" >&2
      exit 1
      ;;
  esac
fi
