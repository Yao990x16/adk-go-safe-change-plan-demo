package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/genai"
)

const defaultPrompt = "模拟服务 checkout-api CPU 持续偏高，给一个处理方案"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := runCLI(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("safe-change-plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	prompt := flags.String("prompt", defaultPrompt, "fictional scenario (one service per request)")
	mode := flags.String("mode", "offline", "offline (no network), gemini, or qwen (explicit model API opt-in)")
	modelName := flags.String("model", "", "Gemini model ID; required with -mode gemini")
	trace := flags.Bool("trace", false, "print ADK tool calls and Qwen model timings to stderr")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("use -prompt for the scenario; positional arguments are unsupported")
	}
	if strings.TrimSpace(*prompt) == "" || len(*prompt) > 4096 {
		return errors.New("prompt must contain 1–4096 bytes")
	}
	timeout := 60 * time.Second
	if *mode == "qwen" {
		timeout = qwenRunTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	llm, err := configuredModel(ctx, *mode, *modelName)
	if err != nil {
		return err
	}
	start := time.Now()
	result, err := runDemoWithOptions(ctx, *prompt, llm, demoOptions{finishWithEvidence: *mode == "qwen"})
	elapsed := time.Since(start)
	if err != nil {
		// Provider errors can include request bodies and endpoint details.
		switch *mode {
		case "gemini":
			return errors.New("Gemini run failed or evidence policy rejected the response; no report generated")
		case "qwen":
			return qwenRunError(err)
		}
		return err
	}
	if *trace {
		data, err := json.Marshal(result.Calls)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stderr, "mode=%s tool_calls=%s\n", *mode, data); err != nil {
			return err
		}
		if *mode == "qwen" {
			if _, err := fmt.Fprintf(stderr, "mode=qwen wall_ms=%d model_calls=%d model_ms=%v\n", elapsed.Milliseconds(), len(result.ModelDurations), milliseconds(result.ModelDurations)); err != nil {
				return err
			}
		}
	}
	_, err = io.WriteString(stdout, result.Text)
	return err
}

func configuredModel(ctx context.Context, mode, name string) (model.LLM, error) {
	switch mode {
	case "offline":
		if name != "" {
			return nil, errors.New("-model is only valid with -mode gemini")
		}
		return offlineModel{}, nil
	case "qwen":
		if name != "" {
			return nil, errors.New("use QWEN_MODEL for -mode qwen; -model is only valid with -mode gemini")
		}
		return newQwenModel(ctx, os.Getenv("QWEN_BASE_URL"), os.Getenv("QWEN_MODEL"))
	case "gemini":
		if !regexp.MustCompile(`^gemini-[a-z0-9.-]+$`).MatchString(name) {
			return nil, errors.New("-mode gemini requires a Gemini model ID via -model")
		}
		key := os.Getenv("GEMINI_API_KEY")
		if strings.TrimSpace(key) == "" {
			return nil, errors.New("GEMINI_API_KEY is required only for explicitly enabled Gemini mode")
		}
		// Explicit backend, endpoint and proxy-free client prevent ambient SDK
		// endpoint/proxy configuration from redirecting this optional request.
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		client := &http.Client{Transport: transport, Timeout: 45 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		llm, err := gemini.NewModel(ctx, name, &genai.ClientConfig{
			APIKey: key, Backend: genai.BackendGeminiAPI, HTTPClient: client,
			HTTPOptions: genai.HTTPOptions{BaseURL: "https://generativelanguage.googleapis.com/"},
		})
		if err != nil {
			return nil, errors.New("could not initialize Gemini client")
		}
		return llm, nil
	default:
		return nil, errors.New("-mode must be offline, gemini or qwen")
	}
}
