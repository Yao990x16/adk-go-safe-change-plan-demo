package main

import (
	"context"
	"encoding/json"
	"iter"
	"regexp"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// offlineModel is a deterministic protocol adapter, NOT an LLM. ADK still
// dispatches real function tools and returns their results through its Runner.
type offlineModel struct{}

func (offlineModel) Name() string { return "offline-deterministic-v1" }

var identifier = regexp.MustCompile("[A-Za-z0-9_][A-Za-z0-9_-]*")

// The intentionally small input grammar accepts service identifiers containing
// a hyphen, or an identifier immediately after 服务/service. Multiple distinct
// candidates require clarification. No substring matching against known names.
func selectService(prompt string) (string, string) {
	if strings.ContainsAny(prompt, "/\\\x00") {
		return "", "不确定，需要人工确认精确服务名；输入不能包含路径。"
	}
	seen := make(map[string]bool)
	for _, name := range identifier.FindAllString(prompt, -1) {
		if strings.Contains(name, "-") {
			seen[name] = true
		}
	}
	for _, prefix := range []string{"服务", "service "} {
		for offset := 0; offset < len(prompt); {
			i := strings.Index(prompt[offset:], prefix)
			if i < 0 {
				break
			}
			offset += i + len(prefix)
			rest := strings.TrimLeft(prompt[offset:], " `\"：:")
			if loc := identifier.FindStringIndex(rest); loc != nil && loc[0] == 0 {
				seen[rest[:loc[1]]] = true
			}
		}
	}
	if len(seen) == 0 {
		return "", "不确定，需要人工确认唯一服务名，例如 checkout-api 或 image-worker。"
	}
	if len(seen) != 1 {
		return "", "不确定，需要人工确认唯一目标服务；一次只处理一个服务。"
	}
	for name := range seen {
		if !serviceName.MatchString(name) {
			return "", "不确定，需要人工确认合法的精确服务名。"
		}
		return name, ""
	}
	panic("unreachable")
}

func (offlineModel) GenerateContent(ctx context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(nil, err)
			return
		}
		var prompt string
		var inspection *InspectResult
		books := make(map[string]bool)
		for _, content := range req.Contents {
			for _, p := range content.Parts {
				if content.Role == string(genai.RoleUser) && p.Text != "" {
					prompt = p.Text
				}
				if p.FunctionResponse == nil {
					continue
				}
				data, err := json.Marshal(p.FunctionResponse.Response)
				if err != nil {
					yield(nil, err)
					return
				}
				switch p.FunctionResponse.Name {
				case "inspect_service":
					var result InspectResult
					if err := json.Unmarshal(data, &result); err != nil {
						yield(nil, err)
						return
					}
					inspection = &result
				case "read_runbook":
					var result RunbookResult
					if err := json.Unmarshal(data, &result); err != nil {
						yield(nil, err)
						return
					}
					if result.Status == "ok" {
						books[result.Topic] = true
					}
				}
			}
		}
		parts := []*genai.Part{{Text: "只读采集完成"}}
		target, _ := selectService(prompt)
		if target != "" && inspection == nil {
			parts = []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "inspect_service", Args: map[string]any{"service": target}}}}
		} else if inspection != nil && inspection.Status == "ok" {
			for _, topic := range requiredTopics(inspection.Service) {
				if !books[topic] {
					parts = []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "read_runbook", Args: map[string]any{"topic": topic}}}}
					break
				}
			}
		}
		yield(&model.LLMResponse{Content: &genai.Content{Role: string(genai.RoleModel), Parts: parts}, TurnComplete: true}, nil)
	}
}
