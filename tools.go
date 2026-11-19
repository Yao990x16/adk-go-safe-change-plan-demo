package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// Compiled snapshots cannot follow symlinks or read caller-controlled paths.
//
//go:embed testdata/services.json runbooks/*.md
var fixtures embed.FS

var serviceName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type InspectArgs struct {
	Service string `json:"service" jsonschema:"Exact fictional service name: checkout-api or image-worker. Never a file path."`
}

type Service struct {
	Name          string `json:"name"`
	Fictional     bool   `json:"fictional"`
	WindowMinutes int    `json:"window_minutes"`
	CPUPercent    int    `json:"cpu_percent"`
	Restarts15m   int    `json:"restarts_15m"`
	State         string `json:"state"`
	QueueDepth    int    `json:"queue_depth"`
	QueueBacklog  bool   `json:"queue_backlog"`
}

type InspectResult struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Service *Service `json:"service,omitempty"`
}

type RunbookArgs struct {
	Topic string `json:"topic" jsonschema:"Exact allowlisted topic: high_cpu, pod_crashloop or queue_backlog. No extensions or paths."`
}

type RunbookResult struct {
	Status string `json:"status"`
	Topic  string `json:"topic"`
	Text   string `json:"text"`
}

// inspectService reads only the embedded fictional snapshot. Unknown names never
// fall back to another service. Both validation layers also apply outside ADK.
func inspectService(_ agent.Context, args InspectArgs) (InspectResult, error) {
	if !serviceName.MatchString(args.Service) {
		return InspectResult{Status: "invalid_argument", Message: "服务名格式非法；仅接受精确名称，不接受路径。"}, nil
	}
	switch args.Service {
	case "checkout-api", "image-worker":
	default:
		return InspectResult{Status: "unknown_service", Message: "未知服务；不确定，需要人工确认服务名。"}, nil
	}
	data, err := fixtures.ReadFile("testdata/services.json")
	if err != nil {
		return InspectResult{}, fmt.Errorf("read embedded services: %w", err)
	}
	var services map[string]Service
	if err := json.Unmarshal(data, &services); err != nil {
		return InspectResult{}, fmt.Errorf("decode embedded services: %w", err)
	}
	s, ok := services[args.Service]
	if !ok || !s.Fictional || s.Name != args.Service || s.WindowMinutes != 15 || s.CPUPercent < 0 || s.CPUPercent > 100 || s.Restarts15m < 0 || s.QueueDepth < 0 {
		return InspectResult{}, fmt.Errorf("invalid fictional fixture for %s", args.Service)
	}
	return InspectResult{Status: "ok", Message: "仅为仓库内虚构快照，不代表任何真实系统。", Service: &s}, nil
}

// readRunbook selects constant paths; user input is never joined into a path.
func readRunbook(_ agent.Context, args RunbookArgs) (RunbookResult, error) {
	var path string
	switch args.Topic {
	case "high_cpu":
		path = "runbooks/high_cpu.md"
	case "pod_crashloop":
		path = "runbooks/pod_crashloop.md"
	case "queue_backlog":
		path = "runbooks/queue_backlog.md"
	default:
		return RunbookResult{Status: "invalid_argument", Topic: "", Text: ""}, nil
	}
	data, err := fixtures.ReadFile(path)
	if err != nil {
		return RunbookResult{}, fmt.Errorf("read embedded Runbook: %w", err)
	}
	return RunbookResult{Status: "ok", Topic: args.Topic, Text: string(data)}, nil
}

func newTools() ([]tool.Tool, error) {
	inspect, err := functiontool.New(functiontool.Config{
		Name:        "inspect_service",
		Description: "只读：读取编译进程序的虚构服务快照。service 仅支持 checkout-api、image-worker；未知服务返回 unknown_service。禁止路径。必须先调用本工具才能断言服务状态；不连接任何真实系统。",
	}, inspectService)
	if err != nil {
		return nil, err
	}
	book, err := functiontool.New(functiontool.Config{
		Name:        "read_runbook",
		Description: "只读：读取内嵌通用 Runbook。topic 仅支持 high_cpu、pod_crashloop、queue_backlog；不接受路径或扩展名。应先 inspect_service，再依据返回指标选 topic。内容仅供人工评审，不能执行操作。",
	}, readRunbook)
	if err != nil {
		return nil, err
	}
	return []tool.Tool{inspect, book}, nil
}
