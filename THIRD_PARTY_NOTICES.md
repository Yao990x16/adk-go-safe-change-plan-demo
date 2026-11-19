# Third-party notices

The project source is Apache-2.0. The table below records the root licenses of
all 34 external modules used by `go list -deps .` on Linux/amd64 with Go 1.27.2
on 2026-11-19. These licenses permit use in this Apache-2.0 demo, subject to
retaining their copyright, license and notice terms. They do not relicense the dependencies.

The corresponding root LICENSE/NOTICE texts are preserved in
[docs/third-party-licenses.txt](docs/third-party-licenses.txt). Dependencies are
resolved by Go modules; no dependency implementation source is vendored here.
ADK's differently licensed internal/httprr test helper is not imported.
This is an inventory of the demo's actual build, not every optional package
in ADK's complete module graph. Recheck package-level licenses and the complete
binary distribution if you change dependencies, build targets or redistribute binaries.

| Module | Pinned version | Root license |
| --- | --- | --- |
| `cloud.google.com/go` | `v0.123.0` | Apache-2.0 |
| `cloud.google.com/go/auth` | `v0.24.1` | Apache-2.0 |
| `cloud.google.com/go/compute/metadata` | `v0.10.0` | Apache-2.0 |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | MIT |
| `github.com/felixge/httpsnoop` | `v1.1.0` | MIT |
| `github.com/go-logr/logr` | `v1.4.4` | Apache-2.0 |
| `github.com/go-logr/stdr` | `v1.2.2` | Apache-2.0 |
| `github.com/google/go-cmp` | `v0.7.0` | BSD-3-Clause |
| `github.com/google/jsonschema-go` | `v0.4.3` | MIT |
| `github.com/google/s2a-go` | `v0.1.11` | Apache-2.0 |
| `github.com/google/safehtml` | `v0.1.0` | BSD-3-Clause |
| `github.com/google/uuid` | `v1.6.0` | BSD-3-Clause |
| `github.com/googleapis/enterprise-certificate-proxy` | `v0.3.24` | Apache-2.0 |
| `github.com/googleapis/gax-go/v2` | `v2.27.0` | BSD-3-Clause |
| `github.com/gorilla/websocket` | `v1.5.3` | BSD-2-Clause |
| `go.opentelemetry.io/auto/sdk` | `v1.2.1` | Apache-2.0 |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | `v0.70.0` | Apache-2.0 |
| `go.opentelemetry.io/otel` | `v1.47.0` | Apache-2.0 |
| `go.opentelemetry.io/otel/log` | `v1.47.0` | Apache-2.0 |
| `go.opentelemetry.io/otel/metric` | `v1.47.0` | Apache-2.0 |
| `go.opentelemetry.io/otel/trace` | `v1.47.0` | Apache-2.0 |
| `golang.org/x/crypto` | `v0.58.0` | BSD-3-Clause |
| `golang.org/x/net` | `v0.61.0` | BSD-3-Clause |
| `golang.org/x/oauth2` | `v0.37.0` | BSD-3-Clause |
| `golang.org/x/sys` | `v0.49.0` | BSD-3-Clause |
| `golang.org/x/text` | `v0.43.0` | BSD-3-Clause |
| `google.golang.org/adk/v2` | `v2.8.0` | Apache-2.0 |
| `google.golang.org/api` | `v0.310.0` | BSD-3-Clause |
| `google.golang.org/genai` | `v1.77.0` | Apache-2.0 |
| `google.golang.org/genproto/googleapis/rpc` | `v0.0.0-20261019180522-ce908d29299b` | Apache-2.0 |
| `google.golang.org/grpc` | `v1.85.0` | Apache-2.0 |
| `google.golang.org/protobuf` | `v1.36.12` | BSD-3-Clause |
| `rsc.io/omap` | `v1.2.0` | BSD-3-Clause |
| `rsc.io/ordered` | `v1.1.1` | BSD-3-Clause |

Go standard-library code is covered by the Go BSD-3-Clause license; see the toolchain LICENSE when distributing a binary.
