package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type denyNetwork struct{ calls int }

func (d *denyNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	d.calls++
	return nil, errors.New("network forbidden in offline test")
}

func TestOfflineCLIWithoutNetwork(t *testing.T) {
	// Trap accidental SDK or HTTP-client use, including ambient credentials.
	trap := &denyNetwork{}
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = trap
	http.DefaultClient = &http.Client{Transport: trap}
	t.Cleanup(func() { http.DefaultTransport, http.DefaultClient = oldTransport, oldClient })
	t.Setenv("GEMINI_API_KEY", "not-a-real-credential")
	t.Setenv("GOOGLE_API_KEY", "not-a-real-credential")
	var out, trace bytes.Buffer
	if err := runCLI(context.Background(), []string{"-trace"}, &out, &trace); err != nil {
		t.Fatal(err)
	}
	if trap.calls != 0 {
		t.Fatalf("offline mode attempted %d network calls", trap.calls)
	}
	if !strings.Contains(out.String(), "CPU 92%") || !strings.Contains(trace.String(), "inspect_service") {
		t.Fatal("CLI did not emit report and actual tool trace")
	}
	if strings.Contains(out.String()+trace.String(), "not-a-real-credential") {
		t.Fatal("credential leaked")
	}
}

func TestCLIConfigurationErrors(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	for _, args := range [][]string{
		{"-mode", "other"}, {"-mode", "gemini"},
		{"-mode", "gemini", "-model", "gemini-test"},
		{"-model", "gemini-test"}, {"-prompt", ""},
		{"-prompt", strings.Repeat("x", 4097)}, {"unexpected positional argument"},
	} {
		var out, stderr bytes.Buffer
		if err := runCLI(context.Background(), args, &out, &stderr); err == nil || out.Len() != 0 {
			t.Fatalf("invalid config accepted: %v", args)
		}
	}
}

func TestGeminiClientInitializationWithoutRequest(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "not-a-real-credential")
	llm, err := configuredModel(context.Background(), "gemini", "gemini-test")
	if err != nil || llm.Name() != "gemini-test" {
		t.Fatalf("optional Gemini adapter initialization: %v", err)
	}
	// Deliberately do not call GenerateContent: no live credentials or API use.
}
