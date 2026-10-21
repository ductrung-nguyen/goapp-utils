# Contributing to goapp-utils

Bug reports, usage questions, and focused pull requests are welcome through [GitHub issues](https://github.com/ndtrunk/goapp-utils/issues) and [pull requests](https://github.com/ndtrunk/goapp-utils/pulls). Include a minimal reproduction for bugs and describe the use case for new helpers.

## Development checks

Use Go 1.27 or newer, with the latest patch release for security fixes. From the repository root:

```sh
go test ./...
go vet ./...
go build ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

The Makefile also provides offline checks (`make test`, `make vet`, `make build`) once dependencies are cached. See `make help` for available targets.

CI runs tests, vet, builds, and [Go vulnerability checks](https://go.dev/doc/security/vuln/) on pull requests and pushes to `main`, with a weekly run to catch newly disclosed issues. Dependencies track current compatible releases; `kube-openapi` stays aligned with stable Kubernetes because its latest development revision uses an incompatible schema API. Vulnerability checks detect known issues, not every possible security flaw.

The release engine and its plugins are built from the separately locked Go sources in `tools/release` and scanned before use. They do not require Node.js or npm.

## Versioning and releases

Use [Conventional Commits](https://www.conventionalcommits.org/) for changes that reach `main`. After the `Go` workflow succeeds for a push to `main`, [Go-native semantic-release](https://github.com/go-semantic-release/semantic-release) determines the next version, creates a `vX.Y.Z` Git tag, and publishes a GitHub release with generated notes. Go consumers use that version tag through `go get`; no manual version-file edits are needed for routine releases.

The first automated release moves the legacy `v0.0.1` baseline to stable `v1.0.0`; the following rules apply to subsequent releases.

| Commit example | Release |
| --- | --- |
| `fix(filewatcher): reattach after file recreation` | Patch |
| `feat(bootstrap): support a new configuration source` | Minor |
| `feat!: change the configuration API` or a `BREAKING CHANGE:` footer | Major |
| `docs: improve configuration examples` or `chore: update tooling` | No release by itself |

For squash merges, use a Conventional Commit PR title and preserve any breaking-change footer in the final squash commit. For merge or rebase merges, keep the individual commit messages conventional. Go dependency updates from Renovate use `fix(deps):` so merged dependency fixes are included in patch releases.

Before a v2 or later major release, update `go.mod` and all imports to the matching `/vN` module path. The release guard rejects a tag whose major version does not match the module path, so an incompatible Go version is not published accidentally.

Repository administrators must allow GitHub Actions to create tags and releases with `GITHUB_TOKEN`; tag rules must permit the release workflow. No personal access token is required. Prefer squash merging with the PR title as the default commit message so the release intent is visible during review.
