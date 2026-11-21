# Qwen Chat Completions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an explicit real Qwen model mode to the existing ADK Go CLI.

**Architecture:** Reuse ADK Go's Chat Completions adapter to translate function calls and results. Configure the Qwen endpoint and model from the environment and strip any inherited authentication headers. Reuse the existing runner, evidence policy and report renderer.

**Tech Stack:** Go `net/http`, ADK Go v2.8.0, `httptest`, Qwen Chat Completions.

---

### Task 1: Configure the model

**Files:** `main.go`, `qwen.go`, `qwen_test.go`.

- [x] Add a local HTTP test that runs the CLI with `-mode qwen`, a loopback `/v1` base URL, and a fake model ID. Assert `/v1/chat/completions`, function declarations, thinking disabled and no authentication headers.
- [x] Implement the CLI branch. Validate `QWEN_BASE_URL` and `QWEN_MODEL`, set a direct HTTP client with a timeout, and select `openaimodel.APIChatCompletions`.
- [x] Script inspection and Runbook calls in the HTTP test. Check that returning inspection evidence triggers a second model request with a matching `tool_call_id`, and that final output uses the fixture instead of the simulated model text.
- [x] Run `go test -run Qwen -v .` and check that a misconfigured endpoint fails before inference.

### Task 2: Verify and document

**Files:** `README.md`, `go.mod`, `go.sum`, `THIRD_PARTY_NOTICES.md`, `docs/third-party-licenses.txt`.

- [x] Document `QWEN_BASE_URL`, `QWEN_MODEL`, the explicit mode, the outbound data and the existing planning boundary. Use a loopback placeholder and a generic model ID in the example.
- [x] Run `go mod tidy`, format edits, run tests and vet, and refresh notices for modules used by the build.
- [x] Use the supplied address through a temporary process environment to validate both fictional services. Check the trace, five report sections, and rollback evidence. Keep address and raw response bodies out of repository files.

The direct Qwen smoke tests passed for CPU and CrashLoop (48 seconds) and for queue backlog (36 seconds). The initial CPU run hit the original time limit. The final configuration caps each model turn at 256 output tokens with a 90-second HTTP deadline and a three-minute CLI deadline.

Execute inline because the user requested implementation. Keep changes available for review in the workspace.
