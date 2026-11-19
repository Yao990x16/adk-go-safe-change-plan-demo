package main

import (
	"strings"
	"testing"
)

func TestInspectService(t *testing.T) {
	for _, tc := range []struct{ name, status string }{
		{"checkout-api", "ok"}, {"image-worker", "ok"},
		{"missing-api", "unknown_service"}, {"", "invalid_argument"},
		{"../checkout-api", "invalid_argument"}, {"/etc/passwd", "invalid_argument"},
		{"checkout-api/../../etc/passwd", "invalid_argument"},
		{"CHECKOUT-API", "invalid_argument"}, {"checkout-api\x00", "invalid_argument"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := inspectService(nil, InspectArgs{Service: tc.name})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.status {
				t.Fatalf("status = %q, want %q", got.Status, tc.status)
			}
			if (got.Service != nil) != (tc.status == "ok") {
				t.Fatal("data present on failure or missing on success")
			}
		})
	}
	got, err := inspectService(nil, InspectArgs{Service: "checkout-api"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Service.CPUPercent != 92 || got.Service.Restarts15m != 3 || got.Service.State != "CrashLoopBackOff" {
		t.Fatalf("unexpected fixture: %+v", got.Service)
	}
	got.Service.CPUPercent = 0
	again, _ := inspectService(nil, InspectArgs{Service: "checkout-api"})
	if again.Service.CPUPercent != 92 {
		t.Fatal("caller mutated fixture")
	}
}

func TestReadRunbook(t *testing.T) {
	for _, topic := range []string{"high_cpu", "pod_crashloop", "queue_backlog"} {
		t.Run(topic, func(t *testing.T) {
			got, err := readRunbook(nil, RunbookArgs{Topic: topic})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "ok" || got.Topic != topic || !strings.Contains(got.Text, "回滚条件") {
				t.Fatalf("bad Runbook: %+v", got)
			}
		})
	}
	for _, topic := range []string{"", "other", "../high_cpu", "../../etc/passwd", "/etc/passwd", "high_cpu.md", "high_cpu/../pod_crashloop", `..\high_cpu`, "high_cpu\x00"} {
		t.Run(topic, func(t *testing.T) {
			got, err := readRunbook(nil, RunbookArgs{Topic: topic})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "invalid_argument" || got.Text != "" {
				t.Fatalf("unsafe topic accepted: %+v", got)
			}
		})
	}
}
