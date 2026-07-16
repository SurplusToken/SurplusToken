package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const chatCompletionStateKey = "chat_completion_state"

type chatGatewayProtocol string

const (
	chatGatewayProtocolResponses chatGatewayProtocol = "responses"
	chatGatewayProtocolMessages  chatGatewayProtocol = "messages"
)

type ChatHandler struct {
	chatService   *service.ChatService
	apiKeyService *service.APIKeyService
}

func NewChatHandler(chatService *service.ChatService, apiKeyService *service.APIKeyService) *ChatHandler {
	return &ChatHandler{chatService: chatService, apiKeyService: apiKeyService}
}

type createChatConversationPayload struct {
	Title           string `json:"title"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	APIKeyID        *int64 `json:"api_key_id"`
}

type updateChatConversationPayload struct {
	Title           *string `json:"title"`
	Model           *string `json:"model"`
	ReasoningEffort *string `json:"reasoning_effort"`
	APIKeyID        *int64  `json:"api_key_id"`
}

type chatCompletionPayload struct {
	APIKeyID        int64  `json:"api_key_id"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	Content         string `json:"content"`
	ClientMessageID string `json:"client_message_id"`
	Stream          *bool  `json:"stream"`
	Attachments     []struct {
		Name    string `json:"name"`
		Mime    string `json:"mime"`
		Kind    string `json:"kind"`
		DataURL string `json:"data_url"`
	} `json:"attachments"`
}

type chatCompletionState struct {
	Prepared *service.PreparedChatCompletion
	Stream   bool
	Protocol chatGatewayProtocol
}

type chatResponsesRequest struct {
	Model      string                      `json:"model"`
	Input      []chatResponsesInputMessage `json:"input"`
	Stream     bool                        `json:"stream"`
	Store      bool                        `json:"store"`
	Reasoning  *chatResponsesReasoning     `json:"reasoning,omitempty"`
	Tools      []chatResponsesTool         `json:"tools"`
	ToolChoice string                      `json:"tool_choice"`
	Include    []string                    `json:"include,omitempty"`
}

type chatResponsesInputMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type chatResponsesContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type chatResponsesReasoning struct {
	Effort string `json:"effort"`
}

type chatResponsesTool struct {
	Type string `json:"type"`
}

type chatMessagesRequest struct {
	Model        string                     `json:"model"`
	MaxTokens    int                        `json:"max_tokens"`
	Messages     []chatMessagesInputMessage `json:"messages"`
	Stream       bool                       `json:"stream"`
	Tools        []chatMessagesTool         `json:"tools,omitempty"`
	OutputConfig *chatMessagesOutputConfig  `json:"output_config,omitempty"`
}

type chatMessagesInputMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type chatMessagesContentPart struct {
	Type   string                   `json:"type"`
	Text   string                   `json:"text,omitempty"`
	Source *chatMessagesImageSource `json:"source,omitempty"`
}

type chatMessagesImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type chatMessagesTool struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type chatMessagesOutputConfig struct {
	Effort string `json:"effort"`
}

func (h *ChatHandler) ListConversations(c *gin.Context) {
	userID, ok := chatUserID(c)
	if !ok {
		return
	}
	page := positiveQueryInt(c.Query("page"), 1)
	pageSize := positiveQueryInt(c.Query("page_size"), 50)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.chatService.ListConversations(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	response.Success(c, response.PaginatedData{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages})
}

func (h *ChatHandler) CreateConversation(c *gin.Context) {
	userID, ok := chatUserID(c)
	if !ok {
		return
	}
	var payload createChatConversationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid chat conversation payload")
		return
	}
	if payload.APIKeyID != nil && !h.ownsAPIKey(c, userID, *payload.APIKeyID) {
		return
	}
	conversation, err := h.chatService.CreateConversation(c.Request.Context(), userID, payload.Title, payload.Model, payload.ReasoningEffort, payload.APIKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, conversation)
}

func (h *ChatHandler) UpdateConversation(c *gin.Context) {
	userID, ok := chatUserID(c)
	if !ok {
		return
	}
	id, ok := chatPathID(c)
	if !ok {
		return
	}
	var payload updateChatConversationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid chat conversation payload")
		return
	}
	if payload.APIKeyID != nil && !h.ownsAPIKey(c, userID, *payload.APIKeyID) {
		return
	}
	conversation, err := h.chatService.UpdateConversation(c.Request.Context(), userID, id, payload.Title, payload.Model, payload.ReasoningEffort, payload.APIKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, conversation)
}

func (h *ChatHandler) DeleteConversation(c *gin.Context) {
	userID, ok := chatUserID(c)
	if !ok {
		return
	}
	id, ok := chatPathID(c)
	if !ok {
		return
	}
	if err := h.chatService.DeleteConversation(c.Request.Context(), userID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *ChatHandler) ListMessages(c *gin.Context) {
	userID, ok := chatUserID(c)
	if !ok {
		return
	}
	id, ok := chatPathID(c)
	if !ok {
		return
	}
	items, err := h.chatService.ListMessages(c.Request.Context(), userID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// PrepareCompletion rewrites the JWT-authenticated chat payload into the
// selected platform's gateway protocol and swaps the JWT header for the owned
// API key before the normal API-key middleware runs.
func (h *ChatHandler) PrepareCompletion(c *gin.Context) {
	userID, ok := chatUserID(c)
	if !ok {
		c.Abort()
		return
	}
	conversationID, ok := chatPathID(c)
	if !ok {
		c.Abort()
		return
	}
	var payload chatCompletionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		chatGatewayError(c, http.StatusBadRequest, "Invalid chat completion payload")
		return
	}
	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), payload.APIKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != userID {
		chatGatewayError(c, http.StatusNotFound, "API key not found")
		return
	}
	if apiKey.Group == nil {
		chatGatewayError(c, http.StatusBadRequest, "API key has no group")
		return
	}
	platform := apiKey.Group.Platform
	protocol, ok := chatProtocolForPlatform(platform)
	if !ok {
		chatGatewayError(c, http.StatusBadRequest, "Unsupported chat platform")
		return
	}
	reasoningEffort, err := service.NormalizeChatReasoningEffort(payload.ReasoningEffort)
	if err != nil {
		appErr := infraerrors.FromError(err)
		chatGatewayError(c, int(appErr.Code), appErr.Message)
		return
	}
	if !chatReasoningEffortSupported(platform, payload.Model, reasoningEffort) {
		chatGatewayError(c, http.StatusBadRequest, "Reasoning effort is not supported by the selected platform")
		return
	}
	payload.ReasoningEffort = reasoningEffort
	prepared, err := h.chatService.PrepareCompletion(c.Request.Context(), userID, conversationID, payload.APIKeyID, payload.Model, payload.ReasoningEffort, payload.ClientMessageID, payload.Content)
	if err != nil {
		appErr := infraerrors.FromError(err)
		chatGatewayError(c, int(appErr.Code), appErr.Message)
		return
	}

	stream := true
	if payload.Stream != nil {
		stream = *payload.Stream
	}
	var gatewayPayload any
	if protocol == chatGatewayProtocolResponses {
		gatewayPayload = buildChatResponsesRequest(payload, prepared, stream)
	} else {
		gatewayPayload = buildChatMessagesRequest(payload, prepared, stream)
	}
	body, err := json.Marshal(gatewayPayload)
	if err != nil {
		_ = h.failPrepared(prepared, service.ChatMessageStatusError, "Failed to build gateway request")
		chatGatewayError(c, http.StatusInternalServerError, "Failed to build gateway request")
		return
	}
	c.Set(chatCompletionStateKey, &chatCompletionState{Prepared: prepared, Stream: stream, Protocol: protocol})
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer "+apiKey.Key)
	if protocol == chatGatewayProtocolResponses {
		c.Request.URL.Path = "/v1/responses"
	} else {
		c.Request.URL.Path = "/v1/messages"
	}
}

func chatProtocolForPlatform(platform string) (chatGatewayProtocol, bool) {
	switch platform {
	case service.PlatformOpenAI, service.PlatformGrok:
		return chatGatewayProtocolResponses, true
	case service.PlatformAnthropic, service.PlatformGemini, service.PlatformAntigravity:
		return chatGatewayProtocolMessages, true
	default:
		return "", false
	}
}

func chatReasoningEffortSupported(platform, model, effort string) bool {
	if effort == "" {
		return true
	}
	if platform == service.PlatformAntigravity && strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gemini-") {
		platform = service.PlatformGemini
	}
	switch platform {
	case service.PlatformOpenAI:
		return effort == "none" || effort == "minimal" || effort == "low" || effort == "medium" || effort == "high" || effort == "xhigh" || effort == "max" || effort == "ultra"
	case service.PlatformGrok:
		return effort == "none" || effort == "minimal" || effort == "low" || effort == "medium" || effort == "high" || effort == "xhigh" || effort == "max"
	case service.PlatformAnthropic, service.PlatformAntigravity:
		return effort == "low" || effort == "medium" || effort == "high" || effort == "xhigh" || effort == "max"
	case service.PlatformGemini:
		return effort == "minimal" || effort == "low" || effort == "medium" || effort == "high"
	default:
		return false
	}
}

func buildChatResponsesRequest(payload chatCompletionPayload, prepared *service.PreparedChatCompletion, stream bool) chatResponsesRequest {
	input := make([]chatResponsesInputMessage, 0, len(prepared.History))
	for _, message := range prepared.History {
		var content any = message.Content
		if message.ID == prepared.UserMessage.ID && len(payload.Attachments) > 0 {
			parts := []chatResponsesContentPart{{Type: "input_text", Text: message.Content}}
			for _, attachment := range payload.Attachments {
				if attachment.Kind == "image" && strings.HasPrefix(attachment.DataURL, "data:image/") {
					parts = append(parts, chatResponsesContentPart{Type: "input_image", ImageURL: attachment.DataURL})
				}
			}
			content = parts
		}
		input = append(input, chatResponsesInputMessage{Role: message.Role, Content: content})
	}
	request := chatResponsesRequest{
		Model:      payload.Model,
		Input:      input,
		Stream:     stream,
		Store:      false,
		Tools:      []chatResponsesTool{{Type: "web_search"}},
		ToolChoice: "auto",
		Include:    []string{"web_search_call.action.sources"},
	}
	if prepared.Conversation.ReasoningEffort != "" {
		request.Reasoning = &chatResponsesReasoning{Effort: prepared.Conversation.ReasoningEffort}
	}
	return request
}

func buildChatMessagesRequest(payload chatCompletionPayload, prepared *service.PreparedChatCompletion, stream bool) chatMessagesRequest {
	messages := make([]chatMessagesInputMessage, 0, len(prepared.History))
	for _, message := range prepared.History {
		var content any = message.Content
		if message.ID == prepared.UserMessage.ID && len(payload.Attachments) > 0 {
			parts := []chatMessagesContentPart{{Type: "text", Text: message.Content}}
			for _, attachment := range payload.Attachments {
				mediaType, data, ok := parseChatImageDataURL(attachment.DataURL)
				if attachment.Kind != "image" || !ok {
					continue
				}
				parts = append(parts, chatMessagesContentPart{Type: "image", Source: &chatMessagesImageSource{Type: "base64", MediaType: mediaType, Data: data}})
			}
			content = parts
		}
		messages = append(messages, chatMessagesInputMessage{Role: message.Role, Content: content})
	}
	request := chatMessagesRequest{
		Model:     payload.Model,
		MaxTokens: 8192,
		Messages:  messages,
		Stream:    stream,
		Tools:     []chatMessagesTool{{Type: "web_search_20250305", Name: "web_search"}},
	}
	if prepared.Conversation.ReasoningEffort != "" {
		request.OutputConfig = &chatMessagesOutputConfig{Effort: prepared.Conversation.ReasoningEffort}
	}
	return request
}

func parseChatImageDataURL(value string) (mediaType, data string, ok bool) {
	header, data, found := strings.Cut(value, ",")
	if !found || data == "" || !strings.HasPrefix(header, "data:image/") || !strings.HasSuffix(strings.ToLower(header), ";base64") {
		return "", "", false
	}
	mediaType = strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
	if mediaType == "" {
		return "", "", false
	}
	return mediaType, data, true
}

// CaptureCompletion forwards the gateway response unchanged while retaining a
// bounded copy for final persistence after downstream middleware completes.
func (h *ChatHandler) CaptureCompletion(c *gin.Context) {
	value, ok := c.Get(chatCompletionStateKey)
	state, ok := value.(*chatCompletionState)
	if !ok || state == nil || state.Prepared == nil {
		c.Next()
		return
	}
	original := c.Writer
	writer := &chatCaptureWriter{ResponseWriter: original, limit: 16 * 1024 * 1024}
	c.Writer = writer
	c.Next()
	c.Writer = original

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	content, model, requestID, usage, gatewayErr := parseCapturedChatResponse(writer.buf.Bytes(), state.Stream, state.Protocol)
	if headerID := strings.TrimSpace(original.Header().Get("X-Request-ID")); headerID != "" {
		requestID = headerID
	}
	if writer.overflow {
		gatewayErr = "Gateway response was too large to persist"
	}
	if c.Request.Context().Err() != nil {
		if err := h.chatService.FailAssistantMessage(ctx, state.Prepared, service.ChatMessageStatusInterrupted, "Generation was interrupted"); err != nil {
			log.Printf("chat: persist interrupted completion: %v", err)
		}
		return
	}
	if original.Status() >= 400 && gatewayErr == "" {
		gatewayErr = http.StatusText(original.Status())
	}
	if gatewayErr != "" {
		if err := h.chatService.FailAssistantMessage(ctx, state.Prepared, service.ChatMessageStatusError, gatewayErr); err != nil {
			log.Printf("chat: persist failed completion: %v", err)
		}
		return
	}
	if model == "" {
		model = state.Prepared.AssistantMessage.Model
	}
	if err := h.chatService.CompleteAssistantMessage(ctx, state.Prepared, content, model, requestID, usage); err != nil {
		log.Printf("chat: persist completed response: %v", err)
	}
}

func (h *ChatHandler) ownsAPIKey(c *gin.Context, userID, apiKeyID int64) bool {
	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), apiKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != userID {
		response.NotFound(c, "API key not found")
		return false
	}
	return true
}

func (h *ChatHandler) failPrepared(prepared *service.PreparedChatCompletion, status, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return h.chatService.FailAssistantMessage(ctx, prepared, status, message)
}

func chatUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return 0, false
	}
	return subject.UserID, true
}

func chatPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid chat conversation ID")
		return 0, false
	}
	return id, true
}

func positiveQueryInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func chatGatewayError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"type": "chat_error", "message": message}})
}

type chatCaptureWriter struct {
	gin.ResponseWriter
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (w *chatCaptureWriter) Write(data []byte) (int, error) {
	w.capture(data)
	return w.ResponseWriter.Write(data)
}

func (w *chatCaptureWriter) WriteString(data string) (int, error) {
	w.capture([]byte(data))
	return w.ResponseWriter.WriteString(data)
}

func (w *chatCaptureWriter) capture(data []byte) {
	if w.limit <= 0 || len(data) == 0 || w.overflow {
		return
	}
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		w.overflow = true
		return
	}
	if len(data) > remaining {
		_, _ = w.buf.Write(data[:remaining])
		w.overflow = true
		return
	}
	_, _ = w.buf.Write(data)
}

type capturedChatCitation struct {
	Type  string `json:"type"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

type capturedChatResponseContent struct {
	Type        string                 `json:"type"`
	Text        string                 `json:"text"`
	Annotations []capturedChatCitation `json:"annotations"`
}

type capturedChatResponseOutput struct {
	Type    string                        `json:"type"`
	Content []capturedChatResponseContent `json:"content"`
	Action  *struct {
		Sources []capturedChatCitation `json:"sources"`
	} `json:"action"`
}

type capturedChatResponse struct {
	ID     string                       `json:"id"`
	Model  string                       `json:"model"`
	Status string                       `json:"status"`
	Output []capturedChatResponseOutput `json:"output"`
	Usage  json.RawMessage              `json:"usage"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type capturedChatResponseEvent struct {
	Type       string                `json:"type"`
	Delta      string                `json:"delta"`
	Response   *capturedChatResponse `json:"response"`
	Annotation *capturedChatCitation `json:"annotation"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

func parseCapturedChatResponse(data []byte, stream bool, protocol chatGatewayProtocol) (content, model, requestID string, usage json.RawMessage, resultErr string) {
	if protocol == chatGatewayProtocolMessages {
		return parseCapturedChatMessagesResponse(data, stream)
	}
	return parseCapturedChatResponsesResponse(data, stream)
}

func parseCapturedChatResponsesResponse(data []byte, stream bool) (content, model, requestID string, usage json.RawMessage, resultErr string) {
	if !stream {
		var result capturedChatResponse
		if err := json.Unmarshal(data, &result); err != nil {
			return "", "", "", nil, "Invalid gateway response"
		}
		if result.Error != nil {
			return "", result.Model, result.ID, result.Usage, result.Error.Message
		}
		content, citations := capturedChatResponseText(&result)
		return appendChatSourceLinks(content, citations), result.Model, result.ID, result.Usage, ""
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var builder strings.Builder
	var citations []capturedChatCitation
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event capturedChatResponseEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		if event.Type == "response.output_text.delta" {
			builder.WriteString(event.Delta)
		}
		if event.Annotation != nil {
			citations = append(citations, *event.Annotation)
		}
		if event.Error != nil && event.Error.Message != "" {
			resultErr = event.Error.Message
		} else if event.Type == "error" && event.Message != "" {
			resultErr = event.Message
		}
		if event.Response == nil {
			continue
		}
		if event.Response.ID != "" {
			requestID = event.Response.ID
		}
		if event.Response.Model != "" {
			model = event.Response.Model
		}
		if len(event.Response.Usage) > 0 && string(event.Response.Usage) != "null" {
			usage = append(json.RawMessage(nil), event.Response.Usage...)
		}
		if event.Response.Error != nil && event.Response.Error.Message != "" {
			resultErr = event.Response.Error.Message
		}
		responseText, responseCitations := capturedChatResponseText(event.Response)
		if builder.Len() == 0 && responseText != "" {
			builder.WriteString(responseText)
		}
		citations = append(citations, responseCitations...)
	}
	if err := scanner.Err(); err != nil {
		return builder.String(), model, "", usage, fmt.Sprintf("Invalid streaming response: %v", err)
	}
	return appendChatSourceLinks(builder.String(), citations), model, requestID, usage, resultErr
}

type capturedChatMessagesContent struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text"`
	Citations []capturedChatCitation `json:"citations"`
	Content   json.RawMessage        `json:"content"`
}

type capturedChatMessagesResponse struct {
	ID      string                        `json:"id"`
	Model   string                        `json:"model"`
	Content []capturedChatMessagesContent `json:"content"`
	Usage   json.RawMessage               `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type capturedChatMessagesEvent struct {
	Type         string                        `json:"type"`
	Message      *capturedChatMessagesResponse `json:"message"`
	ContentBlock *capturedChatMessagesContent  `json:"content_block"`
	Delta        *struct {
		Type     string                `json:"type"`
		Text     string                `json:"text"`
		Citation *capturedChatCitation `json:"citation"`
	} `json:"delta"`
	Usage json.RawMessage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func parseCapturedChatMessagesResponse(data []byte, stream bool) (content, model, requestID string, usage json.RawMessage, resultErr string) {
	if !stream {
		var result capturedChatMessagesResponse
		if err := json.Unmarshal(data, &result); err != nil {
			return "", "", "", nil, "Invalid gateway response"
		}
		if result.Error != nil {
			return "", result.Model, result.ID, result.Usage, result.Error.Message
		}
		text, citations := capturedChatMessagesText(result.Content)
		return appendChatSourceLinks(text, citations), result.Model, result.ID, result.Usage, ""
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var builder strings.Builder
	var citations []capturedChatCitation
	usageFields := make(map[string]json.RawMessage)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event capturedChatMessagesEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		if event.Error != nil && event.Error.Message != "" {
			resultErr = event.Error.Message
		}
		mergeCapturedChatUsage(usageFields, event.Usage)
		if event.Message != nil {
			if event.Message.ID != "" {
				requestID = event.Message.ID
			}
			if event.Message.Model != "" {
				model = event.Message.Model
			}
			mergeCapturedChatUsage(usageFields, event.Message.Usage)
			messageText, messageCitations := capturedChatMessagesText(event.Message.Content)
			if builder.Len() == 0 && messageText != "" {
				builder.WriteString(messageText)
			}
			citations = append(citations, messageCitations...)
			if event.Message.Error != nil && event.Message.Error.Message != "" {
				resultErr = event.Message.Error.Message
			}
		}
		if event.ContentBlock != nil {
			if event.ContentBlock.Text != "" {
				builder.WriteString(event.ContentBlock.Text)
			}
			citations = append(citations, event.ContentBlock.Citations...)
			citations = append(citations, capturedChatMessageSources(event.ContentBlock.Content)...)
		}
		if event.Delta != nil {
			if event.Delta.Type == "text_delta" {
				builder.WriteString(event.Delta.Text)
			}
			if event.Delta.Citation != nil {
				citations = append(citations, *event.Delta.Citation)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return builder.String(), model, requestID, marshalCapturedChatUsage(usageFields), fmt.Sprintf("Invalid streaming response: %v", err)
	}
	return appendChatSourceLinks(builder.String(), citations), model, requestID, marshalCapturedChatUsage(usageFields), resultErr
}

func capturedChatMessagesText(blocks []capturedChatMessagesContent) (string, []capturedChatCitation) {
	var builder strings.Builder
	var citations []capturedChatCitation
	for _, block := range blocks {
		if block.Type == "text" {
			builder.WriteString(block.Text)
		}
		citations = append(citations, block.Citations...)
		citations = append(citations, capturedChatMessageSources(block.Content)...)
	}
	return builder.String(), citations
}

func capturedChatMessageSources(raw json.RawMessage) []capturedChatCitation {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var sources []capturedChatCitation
	if err := json.Unmarshal(raw, &sources); err == nil {
		return sources
	}
	var source capturedChatCitation
	if err := json.Unmarshal(raw, &source); err == nil && source.URL != "" {
		return []capturedChatCitation{source}
	}
	return nil
}

func mergeCapturedChatUsage(fields map[string]json.RawMessage, raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var incoming map[string]json.RawMessage
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return
	}
	for key, value := range incoming {
		if previous, exists := fields[key]; exists && capturedChatJSONNumberIsZero(value) && !capturedChatJSONNumberIsZero(previous) {
			continue
		}
		fields[key] = append(json.RawMessage(nil), value...)
	}
}

func capturedChatJSONNumberIsZero(raw json.RawMessage) bool {
	var value float64
	return json.Unmarshal(raw, &value) == nil && value == 0
}

func marshalCapturedChatUsage(fields map[string]json.RawMessage) json.RawMessage {
	if len(fields) == 0 {
		return nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return encoded
}

func capturedChatResponseText(response *capturedChatResponse) (string, []capturedChatCitation) {
	if response == nil {
		return "", nil
	}
	var builder strings.Builder
	var citations []capturedChatCitation
	for _, output := range response.Output {
		for _, part := range output.Content {
			if part.Type == "output_text" {
				builder.WriteString(part.Text)
			}
			citations = append(citations, part.Annotations...)
		}
		if output.Action != nil {
			citations = append(citations, output.Action.Sources...)
		}
	}
	return builder.String(), citations
}

func appendChatSourceLinks(content string, citations []capturedChatCitation) string {
	seen := make(map[string]struct{}, len(citations))
	links := make([]string, 0, len(citations))
	for _, citation := range citations {
		rawURL := strings.TrimSpace(citation.URL)
		parsed, err := url.Parse(rawURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			continue
		}
		if _, exists := seen[rawURL]; exists {
			continue
		}
		seen[rawURL] = struct{}{}
		title := strings.TrimSpace(citation.Title)
		if title == "" {
			title = parsed.Host
		}
		title = strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]").Replace(title)
		links = append(links, fmt.Sprintf("- [%s](<%s>)", title, strings.ReplaceAll(rawURL, ">", "%3E")))
	}
	if len(links) == 0 {
		return content
	}
	return strings.TrimSpace(content) + "\n\n### Sources\n" + strings.Join(links, "\n")
}
