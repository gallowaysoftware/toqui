package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewOpenAIProvider_DefaultBaseURL(t *testing.T) {
	p := NewOpenAIProvider("test-key", "")
	if p.baseURL != openAIDefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", p.baseURL, openAIDefaultBaseURL)
	}
	if p.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openai")
	}
}

func TestNewOpenAIProvider_TrimsTrailingSlash(t *testing.T) {
	p := NewOpenAIProvider("test-key", "http://localhost:11434/v1/")
	if p.baseURL != "http://localhost:11434/v1" {
		t.Errorf("baseURL = %q, want trimmed value", p.baseURL)
	}
}

func TestNewOpenAIProvider_CustomBaseURL(t *testing.T) {
	// This is the core "Ollama / OpenRouter / vLLM should just work" check —
	// any baseURL the operator provides is honoured verbatim and the request
	// is POSTed to ${baseURL}/chat/completions.
	cases := []string{
		"https://openrouter.ai/api/v1",
		"http://localhost:11434/v1",
		"http://localhost:8000/v1",
	}
	for _, base := range cases {
		t.Run(base, func(t *testing.T) {
			p := NewOpenAIProvider("k", base)
			if p.baseURL != base {
				t.Errorf("baseURL = %q, want %q", p.baseURL, base)
			}
		})
	}
}

func TestOpenAIBuildRequest_SystemPromptBecomesSystemMessage(t *testing.T) {
	p := NewOpenAIProvider("k", "")
	body := p.buildRequest(&ChatRequest{
		SystemPrompt: "You are helpful.",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
	})

	messages := body["messages"].([]map[string]any)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(messages))
	}
	if messages[0]["role"] != "system" || messages[0]["content"] != "You are helpful." {
		t.Errorf("first message should be the system prompt, got %#v", messages[0])
	}
	if messages[1]["role"] != "user" || messages[1]["content"] != "Hi" {
		t.Errorf("second message should be the user message, got %#v", messages[1])
	}
}

func TestOpenAIBuildRequest_StreamAndStreamOptions(t *testing.T) {
	p := NewOpenAIProvider("k", "")
	body := p.buildRequest(&ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if body["stream"] != true {
		t.Error("stream should always be true")
	}
	so, ok := body["stream_options"].(map[string]any)
	if !ok || so["include_usage"] != true {
		t.Errorf("expected stream_options.include_usage=true, got %#v", body["stream_options"])
	}
}

func TestOpenAIBuildRequest_DefaultMaxTokens(t *testing.T) {
	p := NewOpenAIProvider("k", "")
	body := p.buildRequest(&ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if body["max_tokens"] != 4096 {
		t.Errorf("default max_tokens = %v, want 4096", body["max_tokens"])
	}
}

func TestOpenAIBuildRequest_Tools(t *testing.T) {
	p := NewOpenAIProvider("k", "")
	body := p.buildRequest(&ChatRequest{
		Messages: []Message{{Role: "user", Content: "search"}},
		Tools: []ToolDefinition{
			{
				Name:        "web_search",
				Description: "Search the web",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
			},
		},
	})

	if body["tool_choice"] != "auto" {
		t.Errorf("tool_choice = %v, want %q", body["tool_choice"], "auto")
	}

	tools, ok := body["tools"].([]map[string]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools shape wrong: %#v", body["tools"])
	}
	if tools[0]["type"] != "function" {
		t.Errorf("tool type = %v, want %q", tools[0]["type"], "function")
	}
	fn, ok := tools[0]["function"].(map[string]any)
	if !ok {
		t.Fatalf("tool function missing: %#v", tools[0])
	}
	if fn["name"] != "web_search" {
		t.Errorf("tool name = %v, want %q", fn["name"], "web_search")
	}
	if fn["description"] != "Search the web" {
		t.Errorf("tool description = %v", fn["description"])
	}
	if _, ok := fn["parameters"]; !ok {
		t.Error("tool function missing parameters")
	}
}

func TestOpenAIBuildRequest_ToolResultsBecomeToolMessages(t *testing.T) {
	p := NewOpenAIProvider("k", "")
	body := p.buildRequest(&ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "find a hotel"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "search_hotels", Arguments: `{"city":"Paris"}`},
				},
			},
			{
				Role: "user",
				ToolResults: []ToolResult{
					{ToolCallID: "call_1", Name: "search_hotels", Content: `{"hotels":[]}`},
				},
			},
		},
	})

	messages := body["messages"].([]map[string]any)
	// Expect: user, assistant(tool_calls), tool
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d: %#v", len(messages), messages)
	}

	assistant := messages[1]
	if assistant["role"] != "assistant" {
		t.Fatalf("expected assistant role, got %v", assistant["role"])
	}
	tcs, ok := assistant["tool_calls"].([]map[string]any)
	if !ok || len(tcs) != 1 {
		t.Fatalf("tool_calls shape wrong: %#v", assistant["tool_calls"])
	}
	if tcs[0]["id"] != "call_1" || tcs[0]["type"] != "function" {
		t.Errorf("tool_call envelope wrong: %#v", tcs[0])
	}
	fn := tcs[0]["function"].(map[string]any)
	if fn["name"] != "search_hotels" || fn["arguments"] != `{"city":"Paris"}` {
		t.Errorf("tool_call function wrong: %#v", fn)
	}

	tool := messages[2]
	if tool["role"] != "tool" || tool["tool_call_id"] != "call_1" || tool["content"] != `{"hotels":[]}` {
		t.Errorf("tool message wrong: %#v", tool)
	}
}

func TestOpenAIBuildRequest_MultimodalContentBlocks(t *testing.T) {
	p := NewOpenAIProvider("k", "")
	body := p.buildRequest(&ChatRequest{
		Messages: []Message{
			{
				Role: "user",
				ContentBlocks: []ContentBlock{
					{Type: "text", Text: "what's in this picture?"},
					{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: "image/jpeg",
							Data:      "AAAA",
						},
					},
				},
			},
		},
	})

	messages := body["messages"].([]map[string]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	parts, ok := messages[0]["content"].([]map[string]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("multimodal parts shape wrong: %#v", messages[0]["content"])
	}
	if parts[0]["type"] != "text" || parts[0]["text"] != "what's in this picture?" {
		t.Errorf("text part wrong: %#v", parts[0])
	}
	if parts[1]["type"] != "image_url" {
		t.Errorf("image part type wrong: %#v", parts[1])
	}
	imgURL, ok := parts[1]["image_url"].(map[string]any)
	if !ok {
		t.Fatalf("image_url shape wrong: %#v", parts[1])
	}
	url, _ := imgURL["url"].(string)
	if !strings.HasPrefix(url, "data:image/jpeg;base64,") || !strings.HasSuffix(url, "AAAA") {
		t.Errorf("image data URL malformed: %q", url)
	}
}

func TestOpenAIResolveModel_Tiers(t *testing.T) {
	t.Setenv("OPENAI_MODEL_FAST", "")
	t.Setenv("OPENAI_MODEL_SMART", "")
	t.Setenv("OPENAI_MODEL_BEST", "")

	p := NewOpenAIProvider("k", "")

	cases := []struct {
		tier ModelTier
		want string
	}{
		{ModelTierFast, openAIModels[ModelTierFast]},
		{ModelTierSmart, openAIModels[ModelTierSmart]},
		{ModelTierBest, openAIModels[ModelTierBest]},
		{"", openAIModels[ModelTierSmart]}, // default
	}
	for _, c := range cases {
		got := p.resolveModel(&ChatRequest{ModelTier: c.tier})
		if got != c.want {
			t.Errorf("resolveModel(%q) = %q, want %q", c.tier, got, c.want)
		}
	}
}

func TestMapOpenAIStopReason(t *testing.T) {
	cases := map[string]string{
		"stop":           "end_turn",
		"length":         "end_turn",
		"content_filter": "end_turn",
		"tool_calls":     "tool_use",
		"":               "",
		"weird":          "weird",
	}
	for in, want := range cases {
		if got := mapOpenAIStopReason(in); got != want {
			t.Errorf("mapOpenAIStopReason(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- Streaming integration tests --------------------------------------------

// sseServer returns a httptest server that serves chunks back-to-back as
// "data: <line>\n\n" frames plus a trailing "data: [DONE]\n\n".
func sseServer(t *testing.T, chunks []string, capture *http.Request) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			*capture = *r.Clone(r.Context())
			capture.Body = io.NopCloser(strings.NewReader(string(body)))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, c := range chunks {
			_, _ = io.WriteString(w, "data: "+c+"\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	return srv
}

func drainEvents(t *testing.T, ch <-chan Event) []Event {
	t.Helper()
	var out []Event
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

func TestChatStream_TextOnly(t *testing.T) {
	chunks := []string{
		`{"choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
		`{"choices":[{"index":0,"delta":{"content":", world"}}]}`,
		`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`{"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":4}}`,
	}
	srv := sseServer(t, chunks, nil)
	defer srv.Close()

	p := NewOpenAIProvider("k", srv.URL)
	ch, err := p.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	events := drainEvents(t, ch)

	var text strings.Builder
	var done *Event
	for i := range events {
		ev := events[i]
		switch ev.Type {
		case EventTextDelta:
			text.WriteString(ev.Text)
		case EventDone:
			done = &events[i]
		case EventToolCall:
			t.Errorf("unexpected tool call in text-only stream: %+v", ev.Tool)
		case EventError:
			t.Errorf("unexpected error: %v", ev.Error)
		}
	}

	if text.String() != "Hello, world" {
		t.Errorf("text = %q, want %q", text.String(), "Hello, world")
	}
	if done == nil {
		t.Fatal("missing EventDone")
	}
	if done.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", done.StopReason, "end_turn")
	}
	if done.Usage == nil || done.Usage.InputTokens != 12 || done.Usage.OutputTokens != 4 {
		t.Errorf("usage = %+v, want input=12 output=4", done.Usage)
	}
}

func TestChatStream_ToolCall_AccumulatesFragmentedArguments(t *testing.T) {
	// This is the trickiest path: OpenAI streams the tool call id+name once,
	// then drips function.arguments as a sequence of partial JSON fragments,
	// then emits finish_reason="tool_calls". Verify we stitch them into one
	// well-formed JSON string and emit a single EventToolCall.
	chunks := []string{
		// First delta: id + function name, no arguments yet.
		`{"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"create_trip","arguments":""}}]}}]}`,
		// Argument fragments — each carries the same index, no id/name.
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"de"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"stinat"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ion\":\"Paris\"}"}}]}}]}`,
		// Finish.
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		// Usage tail.
		`{"choices":[],"usage":{"prompt_tokens":100,"completion_tokens":20}}`,
	}
	srv := sseServer(t, chunks, nil)
	defer srv.Close()

	p := NewOpenAIProvider("k", srv.URL)
	ch, err := p.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "plan a trip"}},
		Tools: []ToolDefinition{
			{Name: "create_trip", Description: "create", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	events := drainEvents(t, ch)

	var toolCalls []*ToolCall
	var done *Event
	for i := range events {
		ev := events[i]
		switch ev.Type {
		case EventToolCall:
			toolCalls = append(toolCalls, ev.Tool)
		case EventDone:
			done = &events[i]
		case EventError:
			t.Errorf("unexpected error: %v", ev.Error)
		}
	}

	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	tc := toolCalls[0]
	if tc.ID != "call_abc" || tc.Name != "create_trip" {
		t.Errorf("tool call id/name wrong: %+v", tc)
	}
	if tc.Arguments != `{"destination":"Paris"}` {
		t.Errorf("tool call arguments = %q, want %q", tc.Arguments, `{"destination":"Paris"}`)
	}

	// Validate that the stitched arguments are well-formed JSON — the whole
	// point of accumulating before emission.
	var got map[string]any
	if err := json.Unmarshal([]byte(tc.Arguments), &got); err != nil {
		t.Errorf("arguments not valid JSON: %v", err)
	}

	if done == nil || done.StopReason != "tool_use" {
		t.Errorf("done event wrong: %+v", done)
	}
}

func TestChatStream_MultipleParallelToolCalls(t *testing.T) {
	// OpenAI supports parallel tool calls in a single turn — make sure we
	// keep them separate by index and emit in original order.
	chunks := []string{
		`{"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"foo","arguments":"{}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"bar","arguments":"{\"x\":1}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
	}
	srv := sseServer(t, chunks, nil)
	defer srv.Close()

	p := NewOpenAIProvider("k", srv.URL)
	ch, err := p.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "do two things"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	events := drainEvents(t, ch)
	var calls []*ToolCall
	for _, ev := range events {
		if ev.Type == EventToolCall {
			calls = append(calls, ev.Tool)
		}
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].ID != "call_a" || calls[0].Name != "foo" {
		t.Errorf("first call wrong: %+v", calls[0])
	}
	if calls[1].ID != "call_b" || calls[1].Name != "bar" || calls[1].Arguments != `{"x":1}` {
		t.Errorf("second call wrong: %+v", calls[1])
	}
}

func TestChatStream_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"bad model"}}`)
	}))
	defer srv.Close()

	p := NewOpenAIProvider("k", srv.URL)
	_, err := p.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention status 400, got: %v", err)
	}
}

func TestChatStream_InvalidSSE(t *testing.T) {
	// Garbage chunks should be skipped (logged + ignored) rather than killing
	// the stream — match Claude's behaviour.
	chunks := []string{
		`not json at all`,
		`{"choices":[{"index":0,"delta":{"content":"survived"}}]}`,
		`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
	}
	srv := sseServer(t, chunks, nil)
	defer srv.Close()

	p := NewOpenAIProvider("k", srv.URL)
	ch, err := p.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var text strings.Builder
	sawDone := false
	for ev := range ch {
		switch ev.Type {
		case EventTextDelta:
			text.WriteString(ev.Text)
		case EventDone:
			sawDone = true
		case EventError:
			t.Errorf("invalid SSE should not surface as error: %v", ev.Error)
		}
	}
	if text.String() != "survived" {
		t.Errorf("text = %q, want %q", text.String(), "survived")
	}
	if !sawDone {
		t.Error("expected EventDone after invalid SSE")
	}
}

func TestChatStream_StreamErrorEnvelope(t *testing.T) {
	// Some compatible servers (Ollama in older builds) send an error envelope
	// inside the SSE stream rather than failing the HTTP request. We should
	// surface it as EventError.
	chunks := []string{
		`{"error":{"message":"model not found","type":"invalid_request"}}`,
	}
	srv := sseServer(t, chunks, nil)
	defer srv.Close()

	p := NewOpenAIProvider("k", srv.URL)
	ch, err := p.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	gotErr := false
	for ev := range ch {
		if ev.Type == EventError {
			gotErr = true
		}
	}
	if !gotErr {
		t.Error("expected EventError from in-stream error envelope")
	}
}

func TestChatStream_SendsAuthHeader(t *testing.T) {
	var captured http.Request
	chunks := []string{
		`{"choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`,
	}
	srv := sseServer(t, chunks, &captured)
	defer srv.Close()

	p := NewOpenAIProvider("secret-token", srv.URL)
	ch, err := p.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	for range ch { //nolint:revive // drain
	}

	auth := captured.Header.Get("Authorization")
	if auth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer secret-token")
	}
	if !strings.HasSuffix(captured.URL.Path, "/chat/completions") {
		t.Errorf("path = %q, want suffix /chat/completions", captured.URL.Path)
	}
}
