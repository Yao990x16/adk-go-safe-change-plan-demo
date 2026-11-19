# Safe Change Plan Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Deliver a locally reproducible ADK Go planning demo using only fictional embedded fixtures and read-only tools.

**Architecture:** ADK llmagent, functiontool and in-memory Runner provide the actual tool loop. The default model adapter is deterministic and offline; its limited routing is explicitly documented. Inputs select exact service names or Runbook topics, never paths. Reports contain five fixed sections and rollback conditions taken from reviewed Runbooks.

**Tech Stack:** Go 1.26.10+, google.golang.org/adk/v2 v2.8.0, Go testing, Apache-2.0.

---

### Task 1: Fixtures and read-only tools

Files: `tools.go`, `tools_test.go`, `testdata/services.json`, `runbooks/high_cpu.md`, `runbooks/pod_crashloop.md`, `runbooks/queue_backlog.md`.

- [x] Write table-driven tests before implementation: exact known services, unknown service, invalid name, each approved topic, traversal/absolute path rejection, returned-copy isolation.
- [x] Run `go test ./...`; expect missing tool symbols until implementation exists.
- [x] Embed fixtures; use exact maps for both service and topic allowlists. Return structured `ok`, `unknown_service`, or `invalid_argument` results, with no data on failure.
- [x] Register exactly `inspect_service` and `read_runbook` via `functiontool.New`; describe inputs and read-only limits explicitly.
- [x] Run `go test ./...`; expect all tool cases to pass.

### Task 2: ADK loop and bounded CLI

Files: `agent.go`, `offline_model.go`, `main.go`, `agent_test.go`.

- [x] Add an offline `model.LLM` adapter: select an unambiguous service, request inspection, choose Runbooks from returned facts, then render five sections from tool responses.
- [x] Add instruction constraints: tool-grounded facts, uncertainty, no executable destructive commands, per-change rollback and human review.
- [x] Use a fresh in-memory session per prompt, a context deadline, and `agent.RunConfig{MaxLLMCalls: 6}`. Capture tool evidence in ADK before/after callbacks and consume final events; propagate errors.
- [x] Support `go run . -prompt '模拟服务 checkout-api CPU 持续偏高，给一个处理方案'` and optional `-trace` to stderr.
- [x] Verify errors/cancellation do not produce a successful report.

### Task 3: Executable acceptance cases

Files: `eval/tool-selection.json`, `agent_test.go`.

- [x] Define CPU+CrashLoop, backlog, unknown, insufficient-input and direct-restart scenarios, plus ambiguous-service and injection cases.
- [x] Load every case in a real Runner test; compare tool names AND arguments, report headings, grounding, rollback conditions and refusal behavior. Clearly label this custom offline test schema, not Agents CLI compatibility or LLM evaluation.
- [x] Run `go test -run TestToolSelection -v ./...`; require every dataset case to pass.

### Task 4: Reproduction and public-readiness

Files: `README.md`, `Makefile`, `.gitignore`, `LICENSE`, `THIRD_PARTY_NOTICES.md`, `docs/demo-output.txt`.

- [x] Document architecture, offline limitation, dependency pin, mock replacement, tests, five-section reports, two captured examples and future model/approval/change-ticket work.
- [x] Add Apache-2.0 license and review bundled module licenses; exclude local configuration, credentials, binaries and transient artifacts.
- [x] Run `gofmt -w .`, `go test ./...`, `go vet ./...`, `go test -race ./...`, and `go build ./...`.
- [x] Save successful terminal output for the CPU scenario and direct-restart refusal; include actual trace and identify offline mode.
- [x] Review repository text and sensitive-pattern hits; do not stage private configuration, publish, push, create a PR or contact anyone.

The user has already requested implementation, so execute the tasks inline and keep all deliverables local. The neutral module path `example.com/adk-go-safe-change-plan-demo` can be renamed after the owner chooses a public account.

Implementation notes: the user selected an offline default plus optional real-model access. Gemini is explicit opt-in, with no real credentials used during validation. Free-form model output is excluded from reports; application rendering uses successful tool results and reviewed Runbooks. All 10 offline dataset cases passed, including injection and direct-restart refusal.
