package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestQwenToolRoundTrip(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "unused-test-key")
	t.Setenv("OPENAI_ORG_ID", "unused-test-org")
	t.Setenv("OPENAI_CUSTOM_HEADERS", "X-Private-Header: unused-test-value")
	t.Setenv("OPENAI_BASE_URL", "https://example.invalid/v1")
	const name = "qwen-test"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		for _, key := range []string{"Authorization", "OpenAI-Organization", "X-Private-Header"} {
			if r.Header.Get(key) != "" {
				t.Errorf("inherited header reached model: %s", key)
			}
		}
		var req struct {
			Model       string   `json:"model"`
			ToolChoice  string   `json:"tool_choice"`
			MaxTokens   int      `json:"max_tokens"`
			Temperature *float64 `json:"temperature"`
			Template    struct {
				Thinking *bool `json:"enable_thinking"`
			} `json:"chat_template_kwargs"`
			Tools []struct {
				Function struct {
					Name       string                     `json:"name"`
					Parameters map[string]json.RawMessage `json:"parameters"`
				} `json:"function"`
			} `json:"tools"`
			Messages []struct {
				Role       string `json:"role"`
				Content    string `json:"content"`
				ToolCallID string `json:"tool_call_id"`
				ToolCalls  []struct {
					ID string `json:"id"`
				} `json:"tool_calls"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if req.Model != name || req.ToolChoice != "auto" || req.MaxTokens != 256 || req.Temperature == nil || *req.Temperature != 0 || req.Template.Thinking == nil || *req.Template.Thinking {
			t.Error("unexpected model or generation settings")
		}
		tools := make(map[string]bool)
		for _, tool := range req.Tools {
			tools[tool.Function.Name] = len(tool.Function.Parameters["properties"]) != 0
		}
		if len(tools) != 2 || !tools["inspect_service"] || !tools["read_runbook"] {
			t.Errorf("unexpected function declarations: %v", tools)
		}
		if len(req.Messages) < 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" || !strings.Contains(req.Messages[1].Content, "image-worker") {
			t.Error("system instruction and user prompt were not transmitted")
		}
		var response string
		switch len(req.Messages) {
		case 2:
			response = `{"message":{"role":"assistant","tool_calls":[{"id":"inspect-1","type":"function","function":{"name":"inspect_service","arguments":"{\"service\":\"image-worker\"}"}}]},"finish_reason":"tool_calls"}`
		case 4:
			call, result := req.Messages[2], req.Messages[3]
			if call.Role != "assistant" || len(call.ToolCalls) != 1 || call.ToolCalls[0].ID != "inspect-1" || result.Role != "tool" || result.ToolCallID != "inspect-1" || !strings.Contains(result.Content, `"queue_depth":1800`) {
				t.Error("inspection result was not returned with the original call ID and fixture data")
			}
			response = `{"message":{"role":"assistant","tool_calls":[{"id":"book-1","type":"function","function":{"name":"read_runbook","arguments":"{\"topic\":\"queue_backlog\"}"}}]},"finish_reason":"tool_calls"}`
		case 6:
			result := req.Messages[5]
			if result.Role != "tool" || result.ToolCallID != "book-1" || !strings.Contains(result.Content, "回滚条件") {
				t.Error("Runbook result was not returned to the model")
			}
			response = `{"message":{"role":"assistant","content":"队列深度 1，已经扩容"},"finish_reason":"stop"}`
		default:
			t.Errorf("unexpected message count %d", len(req.Messages))
			http.Error(w, "unexpected messages", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-test","object":"chat.completion","model":"qwen-test","choices":[` + response + `]}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("QWEN_BASE_URL", server.URL+"/v1")
	t.Setenv("QWEN_MODEL", name)
	const prompt = "模拟服务 image-worker 队列积压，给处理方案"
	ctx := context.Background()
	llm, err := configuredModel(ctx, "qwen", "")
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := runDemo(ctx, prompt, llm)
	if err != nil {
		t.Fatal(err)
	}
	if count := requests.Swap(0); count != 3 || len(baseline.ModelDurations) != 3 {
		t.Fatalf("baseline = %d HTTP requests, %d model timings, want 3 of each", count, len(baseline.ModelDurations))
	}
	if !strings.Contains(baseline.Text, "队列深度 1800") || !strings.Contains(baseline.Text, "回滚条件") || strings.Contains(baseline.Text, "已经扩容") {
		t.Fatalf("unexpected baseline report: %s", baseline.Text)
	}
	var out, trace bytes.Buffer
	if err := runCLI(ctx, []string{"-mode", "qwen", "-trace", "-prompt", prompt}, &out, &trace); err != nil {
		t.Fatal(err)
	}
	if count := requests.Load(); count != 2 {
		t.Fatalf("optimized CLI sent %d HTTP requests, want 2", count)
	}
	if out.String() != baseline.Text {
		t.Fatalf("optimized CLI changed the report: %s", out.String())
	}
	calls, metrics, _ := strings.Cut(trace.String(), "\n")
	if want := `mode=qwen tool_calls=[{"name":"inspect_service","args":{"service":"image-worker"}},{"name":"read_runbook","args":{"topic":"queue_backlog"}}]`; calls != want || !strings.Contains(metrics, "model_calls=2 model_ms=[") {
		t.Fatalf("unexpected trace: %s", trace.String())
	}
}

func TestQwenRequiresExplicitConfiguration(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "https://example.invalid/v1")
	t.Setenv("QWEN_MODEL", "qwen-test")
	for _, base := range []string{"", "file:///v1", "http://user:pass@example.invalid/v1", "https://example.invalid/v1?debug=1", "https://example.invalid/v1/chat/completions"} {
		t.Setenv("QWEN_BASE_URL", base)
		if _, err := configuredModel(context.Background(), "qwen", ""); err == nil {
			t.Fatalf("invalid base URL accepted: %q", base)
		}
	}
	t.Setenv("QWEN_BASE_URL", "https://example.invalid/v1")
	t.Setenv("QWEN_MODEL", "")
	if _, err := configuredModel(context.Background(), "qwen", ""); err == nil {
		t.Fatal("missing model was accepted")
	}
}

func TestQwenHTTPErrorOmitsResponseAndEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "private response detail", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	t.Setenv("QWEN_BASE_URL", server.URL+"/v1/")
	t.Setenv("QWEN_MODEL", "qwen-test")
	var out, stderr bytes.Buffer
	err := runCLI(context.Background(), []string{"-mode", "qwen", "-trace"}, &out, &stderr)
	if err == nil || err.Error() != "Qwen Chat Completions returned HTTP 503" {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("a failed model call must not emit a report or trace")
	}
}
