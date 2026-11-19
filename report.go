package main

import (
	"errors"
	"fmt"
	"strings"
)

var headings = []string{"观察到的事实", "可能原因", "建议变更计划", "风险与回滚方式", "需要人工确认的事项"}

// Only trusted embedded text and successful tool evidence enter the report.
// The model's free-form response and the user's prompt are never echoed.
func renderReport(prompt string, inspection *InspectResult, books map[string]RunbookResult) (string, error) {
	sections := make([][]string, len(headings))
	_, clarification := selectService(prompt)
	sections[4] = append(sections[4], "- 本 Demo 仅提供计划，不执行任何变更；所有数据均为虚构。")
	if requestsExecution(prompt) {
		sections[4] = append(sections[4], "- 拒绝执行：不会直接重启、删除或批量操作 Pod；请先由人工确认范围、审批人与维护窗口。")
	}
	switch {
	case clarification != "":
		sections[0] = append(sections[0], "- 尚未调用 inspect_service，没有服务状态事实。")
		sections[4] = append(sections[4], "- "+clarification)
	case inspection == nil:
		return "", errors.New("missing inspect_service evidence; no report generated")
	case inspection.Status == "unknown_service" || inspection.Status == "invalid_argument":
		sections[0] = append(sections[0], "- inspect_service 未返回服务状态；无法诊断未知或非法服务。")
		sections[4] = append(sections[4], "- 不确定，需要人工确认服务名；不替换为其他服务，不猜测指标。")
	case inspection.Status == "ok" && inspection.Service != nil:
		s := inspection.Service
		sections[0] = append(sections[0], fmt.Sprintf("- 来源：inspect_service / testdata/services.json；虚构服务 %s，观察窗口 %d 分钟。", s.Name, s.WindowMinutes),
			fmt.Sprintf("- CPU %d%%；近 15 分钟重启 %d 次；状态 %s；队列深度 %d；积压标记 %t。", s.CPUPercent, s.Restarts15m, s.State, s.QueueDepth, s.QueueBacklog),
			"- 这是静态快照，不能证明指标在整个窗口内持续偏高，也不能证明指标间的因果关系。")
		for _, topic := range requiredTopics(s) {
			book, ok := books[topic]
			if !ok || book.Status != "ok" || book.Topic != topic {
				return "", fmt.Errorf("missing read_runbook evidence for %s", topic)
			}
			for i := 1; i < len(headings); i++ {
				lines := runbookSection(book.Text, headings[i])
				if len(lines) == 0 {
					return "", fmt.Errorf("Runbook %s missing section %s", topic, headings[i])
				}
				for _, line := range lines {
					if i == 2 && (!strings.Contains(line, "回滚条件：") || !strings.Contains(line, "回滚方式：")) {
						return "", fmt.Errorf("Runbook %s missing per-change rollback", topic)
					}
					sections[i] = append(sections[i], line+" [Runbook: "+topic+"]")
				}
			}
		}
	default:
		return "", errors.New("invalid inspection evidence")
	}
	if len(sections[1]) == 0 {
		sections[1] = []string{"- 不确定，需要人工确认；现有证据不足以判断原因。"}
	}
	if len(sections[2]) == 0 {
		sections[2] = []string{"- 暂不建议变更；先补充服务名和必要证据。回滚条件：不适用；回滚方式：未执行变更，无需回滚。"}
	}
	if len(sections[3]) == 0 {
		sections[3] = []string{"- 缺少证据时变更可能扩大影响；保持现状，待人工评审，不执行操作。"}
	}
	var out strings.Builder
	for i, h := range headings {
		if i > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(h + "\n" + strings.Join(sections[i], "\n"))
	}
	return out.String() + "\n", nil
}

func runbookSection(text, heading string) []string {
	var lines []string
	active := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			active = line == "## "+heading
			continue
		}
		if active && strings.HasPrefix(line, "- ") {
			lines = append(lines, line)
		}
	}
	return lines
}

func requestsExecution(prompt string) bool {
	prompt = strings.ToLower(prompt)
	for _, word := range []string{"重启", "删除", "执行", "restart", "delete", "kubectl", "ssh", "shell"} {
		if strings.Contains(prompt, word) {
			return true
		}
	}
	return false
}
