package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCapturedChatResponseStream(t *testing.T) {
	raw := []byte("event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello \"}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"world\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-test\",\"status\":\"completed\",\"usage\":{\"total_tokens\":7},\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello world\",\"annotations\":[{\"type\":\"url_citation\",\"url\":\"https://openai.com/docs\",\"title\":\"OpenAI Docs\"}]}]}]}}\n\n")

	content, model, requestID, usage, errMessage := parseCapturedChatResponse(raw, true, chatGatewayProtocolResponses)
	require.Empty(t, errMessage)
	require.Equal(t, "hello world\n\n### Sources\n- [OpenAI Docs](<https://openai.com/docs>)", content)
	require.Equal(t, "gpt-test", model)
	require.Equal(t, "resp_1", requestID)
	require.JSONEq(t, `{"total_tokens":7}`, string(usage))
}

func TestParseCapturedChatResponseStreamError(t *testing.T) {
	raw := []byte("event: response.failed\n" +
		"data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"message\":\"upstream failed\"}}}\n\n")
	_, _, _, _, errMessage := parseCapturedChatResponse(raw, true, chatGatewayProtocolResponses)
	require.Equal(t, "upstream failed", errMessage)
}

func TestParseCapturedChatResponseNonStream(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"id":     "resp_2",
		"model":  "gpt-test",
		"status": "completed",
		"output": []any{map[string]any{
			"type": "message",
			"content": []any{map[string]any{
				"type": "output_text",
				"text": "complete",
			}},
		}},
	})
	require.NoError(t, err)
	content, model, requestID, _, errMessage := parseCapturedChatResponse(raw, false, chatGatewayProtocolResponses)
	require.Empty(t, errMessage)
	require.Equal(t, "complete", content)
	require.Equal(t, "gpt-test", model)
	require.Equal(t, "resp_2", requestID)
}

func TestParseCapturedChatMessagesStream(t *testing.T) {
	raw := []byte("event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-test\",\"content\":[],\"usage\":{\"input_tokens\":5}}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello world\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"citations_delta\",\"citation\":{\"type\":\"web_search_result_location\",\"url\":\"https://docs.anthropic.com/search\",\"title\":\"Anthropic Docs\"}}}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":0,\"output_tokens\":3}}\n\n")

	content, model, requestID, usage, errMessage := parseCapturedChatResponse(raw, true, chatGatewayProtocolMessages)
	require.Empty(t, errMessage)
	require.Equal(t, "hello world\n\n### Sources\n- [Anthropic Docs](<https://docs.anthropic.com/search>)", content)
	require.Equal(t, "claude-test", model)
	require.Equal(t, "msg_1", requestID)
	require.JSONEq(t, `{"input_tokens":5,"output_tokens":3}`, string(usage))
}

func TestChatResponsesRequestJSON(t *testing.T) {
	withReasoning, err := json.Marshal(chatResponsesRequest{
		Model:      "gpt-test",
		Input:      []chatResponsesInputMessage{{Role: "user", Content: "hello"}},
		Stream:     true,
		Store:      false,
		Reasoning:  &chatResponsesReasoning{Effort: "high"},
		Tools:      []chatResponsesTool{{Type: "web_search"}},
		ToolChoice: "auto",
		Include:    []string{"web_search_call.action.sources"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"gpt-test",
		"input":[{"role":"user","content":"hello"}],
		"stream":true,
		"store":false,
		"reasoning":{"effort":"high"},
		"tools":[{"type":"web_search"}],
		"tool_choice":"auto",
		"include":["web_search_call.action.sources"]
	}`, string(withReasoning))

	automatic, err := json.Marshal(chatResponsesRequest{Model: "gpt-test", Input: nil, Stream: true})
	require.NoError(t, err)
	require.NotContains(t, string(automatic), "reasoning")
}

func TestChatMessagesRequestJSON(t *testing.T) {
	request, err := json.Marshal(chatMessagesRequest{
		Model:     "claude-test",
		MaxTokens: 8192,
		Messages: []chatMessagesInputMessage{{
			Role: "user",
			Content: []chatMessagesContentPart{
				{Type: "text", Text: "hello"},
				{Type: "image", Source: &chatMessagesImageSource{Type: "base64", MediaType: "image/png", Data: "abc"}},
			},
		}},
		Stream:       true,
		Tools:        []chatMessagesTool{{Type: "web_search_20250305", Name: "web_search"}},
		OutputConfig: &chatMessagesOutputConfig{Effort: "xhigh"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"claude-test",
		"max_tokens":8192,
		"messages":[{"role":"user","content":[
			{"type":"text","text":"hello"},
			{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}
		]}],
		"stream":true,
		"tools":[{"type":"web_search_20250305","name":"web_search"}],
		"output_config":{"effort":"xhigh"}
	}`, string(request))
}

func TestChatPlatformProtocolAndReasoning(t *testing.T) {
	protocol, ok := chatProtocolForPlatform("openai")
	require.True(t, ok)
	require.Equal(t, chatGatewayProtocolResponses, protocol)
	protocol, ok = chatProtocolForPlatform("anthropic")
	require.True(t, ok)
	require.Equal(t, chatGatewayProtocolMessages, protocol)
	require.True(t, chatReasoningEffortSupported("openai", "gpt-5.6-sol", "ultra"))
	require.False(t, chatReasoningEffortSupported("grok", "grok-4", "ultra"))
	require.True(t, chatReasoningEffortSupported("anthropic", "claude-opus-4-8", "xhigh"))
	require.False(t, chatReasoningEffortSupported("anthropic", "claude-opus-4-8", "minimal"))
	require.True(t, chatReasoningEffortSupported("gemini", "gemini-3.5-flash", "minimal"))
	require.False(t, chatReasoningEffortSupported("gemini", "gemini-3.5-flash", "max"))
}
