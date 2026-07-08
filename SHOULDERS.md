# Shoulders

candle is vibecoded: built through iterative, human-directed AI coding sessions.
This file lists the main technologies, libraries, and project infrastructure it
stands on.

## Core Technologies

| Technology | Role |
|------------|------|
| Go 1.26.5 | Implementation language and runtime |
| SQLite | Local persistent index store |
| MCP | Stdio protocol surface for AI agents |
| Graphify | Upstream code graph producer consumed by candle |
| OpenAPI | HTTP API contract layer |
| Protocol Buffers | RPC/schema contract layer |
| GitHub Actions | CI and release automation |
| GoReleaser | Cross-platform binary release packaging |
| mise | Pinned local and CI toolchain management |
| Task | Developer task runner |

## Direct Go Libraries

| Module | Version | Role |
|--------|---------|------|
| `github.com/bufbuild/protocompile` | `v0.14.1` | Protobuf parsing and linking support |
| `github.com/getkin/kin-openapi` | `v0.140.0` | OpenAPI parsing and validation support |
| `github.com/modelcontextprotocol/go-sdk` | `v1.6.1` | MCP server SDK |
| `github.com/spf13/cobra` | `v1.10.2` | CLI command framework |
| `github.com/spf13/viper` | `v1.21.0` | Configuration loading |
| `golang.org/x/mod` | `v0.37.0` | Go module parsing support |
| `google.golang.org/protobuf` | `v1.34.2` | Protobuf runtime/types |
| `modernc.org/sqlite` | `v1.52.0` | Pure-Go SQLite driver |

## Toolchain

| Tool | Version | Role |
|------|---------|------|
| `go` | `1.26.5` | Build, test, vet, module tooling |
| `task` | `3.52.0` | Task runner for build/test/release workflows |
| `golangci-lint` | `2.12.2` | Static analysis and lint gate |
| `goreleaser` | `2.16.0` | Release builds, archives, checksums, GitHub releases |
| `govulncheck` | `latest` via mise | Reachable vulnerability scanning |

## Project Workflows

| Workflow | Role |
|----------|------|
| `go test ./...` | Unit and end-to-end test suite |
| `go test -race -coverprofile=coverage.out -covermode=atomic ./...` | Race-enabled coverage run |
| `go vet ./...` | Go static checks |
| `golangci-lint run` | Lint gate |
| `govulncheck ./...` | Vulnerability gate |
| `goreleaser release --snapshot --clean` | Local and CI release smoke test |
| `goreleaser release --clean` | Tag-driven publish workflow |

## Supporting Go Modules

These are indirect modules recorded in `go.mod` and pulled in by the direct
libraries above.

| Module | Version |
|--------|---------|
| `github.com/dustin/go-humanize` | `v1.0.1` |
| `github.com/fsnotify/fsnotify` | `v1.9.0` |
| `github.com/go-openapi/jsonpointer` | `v0.22.5` |
| `github.com/go-openapi/swag/jsonname` | `v0.25.5` |
| `github.com/go-viper/mapstructure/v2` | `v2.4.0` |
| `github.com/google/jsonschema-go` | `v0.4.3` |
| `github.com/google/uuid` | `v1.6.0` |
| `github.com/inconshreveable/mousetrap` | `v1.1.0` |
| `github.com/mattn/go-isatty` | `v0.0.20` |
| `github.com/ncruces/go-strftime` | `v1.0.0` |
| `github.com/oasdiff/yaml` | `v0.1.0` |
| `github.com/oasdiff/yaml3` | `v0.0.13` |
| `github.com/pelletier/go-toml/v2` | `v2.2.4` |
| `github.com/remyoudompheng/bigfft` | `v0.0.0-20230129092748-24d4a6f8daec` |
| `github.com/sagikazarmark/locafero` | `v0.11.0` |
| `github.com/santhosh-tekuri/jsonschema/v6` | `v6.0.2` |
| `github.com/segmentio/asm` | `v1.1.3` |
| `github.com/segmentio/encoding` | `v0.5.4` |
| `github.com/sourcegraph/conc` | `v0.3.1-0.20240121214520-5f936abd7ae8` |
| `github.com/spf13/afero` | `v1.15.0` |
| `github.com/spf13/cast` | `v1.10.0` |
| `github.com/spf13/pflag` | `v1.0.10` |
| `github.com/subosito/gotenv` | `v1.6.0` |
| `github.com/yosida95/uritemplate/v3` | `v3.0.2` |
| `go.yaml.in/yaml/v3` | `v3.0.4` |
| `golang.org/x/oauth2` | `v0.35.0` |
| `golang.org/x/sync` | `v0.20.0` |
| `golang.org/x/sys` | `v0.42.0` |
| `golang.org/x/text` | `v0.28.0` |
| `modernc.org/libc` | `v1.72.3` |
| `modernc.org/mathutil` | `v1.7.1` |
| `modernc.org/memory` | `v1.11.0` |

## Acknowledgements

candle exists because these ecosystems are usable, composable, and well-tested:
Go, SQLite, MCP, Graphify, OpenAPI, protobuf, the Go module ecosystem, GitHub
Actions, mise, Task, GoReleaser, golangci-lint, and govulncheck.
