package main

import (
	"context"
	"flag"
	"testing"
	"time"
)

var qwenLive = flag.Bool("qwen-live", false, "run the Qwen acceptance cases against the configured server")
var qwenBaseline = flag.Bool("qwen-baseline", false, "keep the final model round for a baseline comparison")

func TestQwenLive(t *testing.T) {
	if !*qwenLive {
		t.Skip("use make eval-qwen to enable live Qwen requests")
	}
	variant := "optimized"
	if *qwenBaseline {
		variant = "baseline"
	}
	cases := loadToolSelectionCases(t)
	for _, id := range []string{"cpu-and-crashloop", "queue-backlog"} {
		tc, ok := cases[id]
		if !ok {
			t.Fatalf("missing live acceptance case %q", id)
		}
		t.Run(id, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), qwenRunTimeout)
			defer cancel()
			llm, err := configuredModel(ctx, "qwen", "")
			if err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			got, err := runDemoWithOptions(ctx, tc.Prompt, llm, demoOptions{finishWithEvidence: !*qwenBaseline})
			elapsed := time.Since(start).Round(time.Millisecond)
			if err != nil {
				t.Fatalf("failed after %s: %v", elapsed, qwenRunError(err))
			}
			count := len(got.ModelDurations)
			t.Logf("variant=%s wall=%s model_calls=%d model_ms=%v", variant, elapsed, count, milliseconds(got.ModelDurations))
			minCalls, maxCalls := 2, len(tc.Calls)
			if *qwenBaseline {
				minCalls++
				maxCalls++
			}
			if count < minCalls || count > maxCalls {
				t.Fatalf("model calls = %d, want %d–%d", count, minCalls, maxCalls)
			}
			assertToolSelection(t, got, tc)
		})
	}
}
