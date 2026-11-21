package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
)

const qwenRunTimeout = 3 * time.Minute

func newQwenModel(ctx context.Context, baseURL, name string) (model.LLM, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" ||
		base.User != nil || base.ForceQuery || base.RawQuery != "" || base.Fragment != "" ||
		strings.TrimSuffix(base.EscapedPath(), "/") != "/v1" {
		return nil, errors.New("QWEN_BASE_URL must be an HTTP(S) base URL ending in /v1, without credentials, query or fragment")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("QWEN_MODEL must contain the full model ID advertised by the server")
	}
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{
		Transport: qwenTransport{transport},
		Timeout:   90 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	llm, err := openaimodel.NewModel(ctx, name, &openaimodel.ClientConfig{
		API:        openaimodel.APIChatCompletions,
		BaseURL:    base.String(),
		HTTPClient: client,
		Options: []option.RequestOption{
			option.WithAPIKey(""),
			option.WithAdminAPIKey(""),
			option.WithMaxRetries(0),
			option.WithJSONSet("tool_choice", "auto"),
			option.WithJSONSet("temperature", 0),
			option.WithJSONSet("max_tokens", 256),
			option.WithJSONSet("chat_template_kwargs.enable_thinking", false),
		},
	})
	if err != nil {
		return nil, errors.New("could not initialize Qwen client")
	}
	return llm, nil
}

// The compatibility client reads default headers from the environment. This
// no-auth service only needs JSON, so allowlist headers at the HTTP boundary.
type qwenTransport struct{ http.RoundTripper }

func (t qwenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clean := req.Clone(req.Context())
	clean.Host = ""
	clean.Header = http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json"}}
	return t.RoundTripper.RoundTrip(clean)
}

func qwenRunError(err error) error {
	var apiErr *openai.Error
	var networkErr net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return errors.New("Qwen run canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("Qwen run timed out")
	case errors.As(err, &apiErr):
		return fmt.Errorf("Qwen Chat Completions returned HTTP %d", apiErr.StatusCode)
	case errors.As(err, &networkErr):
		return errors.New("Qwen connection failed; check QWEN_BASE_URL and network access")
	default:
		return errors.New("Qwen run failed or evidence policy rejected the response; no report generated")
	}
}
