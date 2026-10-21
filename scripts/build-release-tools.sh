#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tools_dir=$repo_root/tools/release
bin_dir=${GOBIN:-${RUNNER_TEMP:-${TMPDIR:-/tmp}/goapp-utils-release-tools}}
plugin_dir=$repo_root/.semrel/$(go env GOOS)_$(go env GOARCH)

mkdir -p "$bin_dir"
go -C "$tools_dir" build -o "$bin_dir/semantic-release" github.com/go-semantic-release/semantic-release/v2/cmd/semantic-release
go -C "$tools_dir" build -o "$bin_dir/govulncheck" golang.org/x/vuln/cmd/govulncheck

build_plugin() {
  name=$1
  version=$2
  package=$3
  target=$plugin_dir/$name/$version/$name
  mkdir -p "$(dirname -- "$target")"
  go -C "$tools_dir" build -o "$target" "$package"
}

build_plugin provider-github 1.19.0 github.com/go-semantic-release/provider-github/cmd/provider-github
build_plugin condition-github 1.11.0 github.com/go-semantic-release/condition-github/cmd/condition-github
build_plugin commit-analyzer-default 1.15.0 github.com/go-semantic-release/commit-analyzer-cz/cmd/commit-analyzer-cz
build_plugin changelog-generator-default 1.15.0 github.com/go-semantic-release/changelog-generator-default/cmd/changelog-generator-default

"$bin_dir/govulncheck" -mode=binary "$bin_dir/semantic-release"
for plugin in "$plugin_dir"/*/*/*; do
  [ -f "$plugin" ] || continue
  "$bin_dir/govulncheck" -mode=binary "$plugin"
done
