# Qwen Live Evaluation and Latency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Reproduce the two Qwen demonstrations and reduce latency after gathering all report evidence.

**Architecture:** Reuse the existing dataset and assertions for the offline and live runs. Time model generation at the `model.LLM` interface. For Qwen, use an ADK callback to finish locally once a successful inspection and every required Runbook are present. Keep the previous model finish path available in the test harness for comparison.

**Tech Stack:** Go testing, ADK Go v2.8.0, Qwen Chat Completions.

---

### Task 1: Capture a baseline

**Files:** `agent.go`, `agent_test.go`, `qwen_live_test.go`, `Makefile`.

- [x] Record `[]time.Duration` for actual `model.LLM.GenerateContent` iterations in `DemoResult`. Begin timing before calling the underlying adapter and finish when the iterator returns.
- [x] Extract shared assertions from `TestToolSelection`. `TestQwenLive` loads `cpu-and-crashloop` and `queue-backlog`, runs a fresh ADK invocation per case, and verifies exact tool arguments, inspection-first ordering, five report sections and rollback markers.
- [x] Gate live evaluation with `-qwen-live`. Add `make eval-qwen` with `-count=1`. Run both live cases sequentially and record their wall times, per-call durations and request counts before changing the finish policy.

### Task 2: Finish when evidence is complete

**Files:** `agent.go`, `main.go`, `agent_test.go`, `qwen_test.go`, `qwen_live_test.go`, `Makefile`.

- [x] Add a failing test requiring three actual model calls for the deterministic CPU fixture where each Runbook is requested separately. Keep the two-runbook requirement in the assertion.
- [x] Register an opt-in `BeforeModelCallback` for Qwen. Under the evidence lock, check for no violation, an `ok` inspection with service and an `ok` Runbook for every `requiredTopics` entry. Return a local final response only when every check passes. Let the existing renderer still validate the final evidence.
- [x] Expose actual request count and timing under Qwen `-trace`. Add a test proving incomplete evidence still reaches the model and the CLI saves an HTTP request when evidence is complete.
- [x] Add `make eval-qwen-baseline` with `-qwen-baseline`. Share evaluation assertions with the optimized run so the same output contract applies to both paths.

### Task 3: Verify and document the measured change

**Files:** `README.md`, `docs/qwen-evaluation.md`.

- [x] Run optimized live evaluation with the same settings used for baseline, then record measurements and request counts with no service address or raw bodies in the repo.
- [x] Document both commands, how to read the metrics, test scope, and the implementation steps. Use placeholder endpoint values in examples.
- [x] Run `make check`, `go test -race ./...`, and `go build ./...`. Review the diffs for internal endpoint details and verify the working tree against the initial Qwen commit.

Both live targets passed. Initial baseline was 39.560 s and 35.308 s. The optimized runs were 10.064 s and 6.702 s. The final baseline target reproduced three calls for both cases at 39.536 s and 35.623 s. The loopback HTTP test verifies three requests versus two and identical report bytes.
