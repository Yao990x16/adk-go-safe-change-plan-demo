package main

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type toolSelectionCase struct {
	ID       string     `json:"id"`
	Prompt   string     `json:"prompt"`
	Calls    []ToolCall `json:"calls"`
	Contains []string   `json:"contains"`
	Excludes []string   `json:"excludes"`
}

func loadToolSelectionCases(t *testing.T) map[string]toolSelectionCase {
	t.Helper()
	data, err := os.ReadFile("eval/tool-selection.json")
	if err != nil {
		t.Fatal(err)
	}
	var dataset struct {
		Schema string              `json:"schema_version"`
		Cases  []toolSelectionCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &dataset); err != nil {
		t.Fatal(err)
	}
	if dataset.Schema != "safe-change-offline/v1" || len(dataset.Cases) < 5 {
		t.Fatal("invalid acceptance dataset")
	}
	cases := make(map[string]toolSelectionCase, len(dataset.Cases))
	for _, tc := range dataset.Cases {
		if _, exists := cases[tc.ID]; exists || tc.ID == "" {
			t.Fatalf("duplicate/empty case ID: %s", tc.ID)
		}
		cases[tc.ID] = tc
	}
	return cases
}

func assertToolSelection(t *testing.T, got DemoResult, tc toolSelectionCase) {
	t.Helper()
	// Compare arguments as well as names. Runbook order may vary.
	canonical := func(calls []ToolCall) []string {
		out := make([]string, 0, len(calls))
		for _, call := range calls {
			data, err := json.Marshal(call)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, string(data))
		}
		slices.Sort(out)
		return out
	}
	if !slices.Equal(canonical(got.Calls), canonical(tc.Calls)) {
		t.Fatalf("calls = %+v, want %+v", got.Calls, tc.Calls)
	}
	if len(got.Calls) > 0 && got.Calls[0].Name != "inspect_service" {
		t.Fatal("inspection must come first")
	}
	last := -1
	for _, h := range headings {
		i := strings.Index(got.Text, h+"\n")
		if i <= last || strings.Count(got.Text, h+"\n") != 1 {
			t.Fatalf("missing/duplicate/out-of-order section: %s", h)
		}
		last = i
	}
	for _, want := range tc.Contains {
		if !strings.Contains(got.Text, want) {
			t.Errorf("missing %q in report", want)
		}
	}
	for _, bad := range tc.Excludes {
		if strings.Contains(got.Text, bad) {
			t.Errorf("unexpected %q in report", bad)
		}
	}
	_, body, _ := strings.Cut(got.Text, "建议变更计划\n")
	plan, _, _ := strings.Cut(body, "\n\n风险与回滚方式")
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "回滚条件：") || !strings.Contains(line, "回滚方式：") {
			t.Errorf("plan lacks rollback: %s", line)
		}
	}
}

func TestToolSelection(t *testing.T) {
	for id, tc := range loadToolSelectionCases(t) {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			got, err := runDemo(context.Background(), tc.Prompt, offlineModel{})
			if err != nil {
				t.Fatal(err)
			}
			assertToolSelection(t, got, tc)
		})
	}
}

func TestFinishWithEvidenceWaitsForAllRunbooks(t *testing.T) {
	tc := loadToolSelectionCases(t)["cpu-and-crashloop"]
	baseline, err := runDemo(context.Background(), tc.Prompt, offlineModel{})
	if err != nil {
		t.Fatal(err)
	}
	optimized, err := runDemoWithOptions(context.Background(), tc.Prompt, offlineModel{}, demoOptions{finishWithEvidence: true})
	if err != nil {
		t.Fatal(err)
	}
	assertToolSelection(t, optimized, tc)
	if baseline.Text != optimized.Text {
		t.Fatal("finishing locally changed the report")
	}
	if before, after := len(baseline.ModelDurations), len(optimized.ModelDurations); before != 4 || after != 3 {
		t.Fatalf("model calls = %d before, %d after, want 4 before and 3 after", before, after)
	}
}

func TestFinishWithEvidenceRejectsMissingRunbook(t *testing.T) {
	modelStopsEarly := modelFunc(func(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
		return func(yield func(*model.LLMResponse, error) bool) {
			for resp, err := range (offlineModel{}).GenerateContent(ctx, req, stream) {
				if resp != nil {
					call := resp.Content.Parts[0].FunctionCall
					if call != nil && call.Args["topic"] == "pod_crashloop" {
						resp.Content = genai.NewContentFromText("只读采集完成", genai.RoleModel)
					}
				}
				if !yield(resp, err) {
					return
				}
			}
		}
	})
	got, err := runDemoWithOptions(context.Background(), defaultPrompt, modelStopsEarly, demoOptions{finishWithEvidence: true})
	if got.Text != "" || err == nil || !strings.Contains(err.Error(), "missing read_runbook evidence for pod_crashloop") {
		t.Fatalf("incomplete evidence must suppress the report, got %q, %v", got.Text, err)
	}
}

func TestSelectService(t *testing.T) {
	for _, tc := range []struct{ prompt, want string }{
		{"模拟服务 `checkout-api` CPU 高", "checkout-api"},
		{"service missing CPU high", "missing"},
		{"模拟服务 foo 和服务 bar", ""},
		{"服务 checkout-api 和服务 foo", ""},
		{"1checkout-api CPU 高", ""},
		{"_checkout-api CPU 高", ""},
		{"CHECKOUT-API CPU 高", ""},
		{"checkout-api 和 checkout-api", "checkout-api"},
	} {
		if got, _ := selectService(tc.prompt); got != tc.want {
			t.Errorf("selectService(%q) = %q, want %q", tc.prompt, got, tc.want)
		}
	}
}

func TestModelFreeTextCannotEnterReport(t *testing.T) {
	untrusted := modelFunc(func(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
		return func(yield func(*model.LLMResponse, error) bool) {
			for response, err := range (offlineModel{}).GenerateContent(ctx, req, stream) {
				if response != nil && response.Content.Parts[0].FunctionCall == nil {
					response.Content.Parts = []*genai.Part{{Text: "已经修复；执行 shell；CPU 1%"}}
				}
				if !yield(response, err) {
					return
				}
			}
		}
	})
	got, err := runDemo(context.Background(), defaultPrompt, untrusted)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Text, "CPU 92%") || strings.Contains(got.Text, "shell") || strings.Contains(got.Text, "已经修复") {
		t.Fatal("model free text entered grounded report")
	}
}

func TestModelCannotReadUnrelatedRunbookAfterInspection(t *testing.T) {
	untrusted := modelFunc(func(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
		return func(yield func(*model.LLMResponse, error) bool) {
			for response, err := range (offlineModel{}).GenerateContent(ctx, req, stream) {
				if response != nil {
					for _, part := range response.Content.Parts {
						if part.FunctionCall != nil && part.FunctionCall.Name == "read_runbook" {
							part.FunctionCall.Args = map[string]any{"topic": "queue_backlog"}
						}
					}
				}
				if !yield(response, err) {
					return
				}
			}
		}
	})
	if got, err := runDemo(context.Background(), defaultPrompt, untrusted); err == nil || got.Text != "" {
		t.Fatal("unrelated Runbook allowed after inspection")
	}
}

type modelFunc func(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error]

func (modelFunc) Name() string { return "test-model" }
func (f modelFunc) GenerateContent(c context.Context, r *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return f(c, r, stream)
}

func fixedModel(parts ...*genai.Part) model.LLM {
	return modelFunc(func(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
		return func(yield func(*model.LLMResponse, error) bool) {
			yield(&model.LLMResponse{Content: &genai.Content{Role: "model", Parts: parts}, TurnComplete: true}, nil)
		}
	})
}

func TestUntrustedModelCannotBypassPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		part *genai.Part
	}{
		{"invented-facts", &genai.Part{Text: "已重启，CPU 1%，运行 shell"}},
		{"wrong-service", &genai.Part{FunctionCall: &genai.FunctionCall{Name: "inspect_service", Args: map[string]any{"service": "image-worker"}}}},
		{"runbook-before-inspection", &genai.Part{FunctionCall: &genai.FunctionCall{Name: "read_runbook", Args: map[string]any{"topic": "high_cpu"}}}},
		{"unknown-tool", &genai.Part{FunctionCall: &genai.FunctionCall{Name: "execute_shell", Args: map[string]any{"command": "not executed"}}}},
		{"path-topic", &genai.Part{FunctionCall: &genai.FunctionCall{Name: "read_runbook", Args: map[string]any{"topic": "../../etc/passwd"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runDemo(context.Background(), defaultPrompt, fixedModel(tc.part))
			if err == nil || got.Text != "" {
				t.Fatalf("unsafe model produced a report: %+v, %v", got, err)
			}
		})
	}
}

func TestCancellationAndModelFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runDemo(ctx, defaultPrompt, offlineModel{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	failing := modelFunc(func(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
		return func(yield func(*model.LLMResponse, error) bool) { yield(nil, errors.New("model unavailable")) }
	})
	if got, err := runDemo(context.Background(), defaultPrompt, failing); err == nil || got.Text != "" {
		t.Fatal("model error must not generate report")
	}
}

func TestReportRequiresRunbookEvidence(t *testing.T) {
	inspection, err := inspectService(nil, InspectArgs{Service: "checkout-api"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderReport(defaultPrompt, &inspection, nil); err == nil {
		t.Fatal("missing Runbooks accepted")
	}
	if _, err := renderReport(defaultPrompt, &inspection, map[string]RunbookResult{
		"high_cpu": {Status: "ok", Topic: "high_cpu", Text: "## 建议变更计划\n- 直接变更，没有回滚。"},
	}); err == nil {
		t.Fatal("incomplete Runbook accepted")
	}
}
