package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	ChatMessageRoleUser      = "user"
	ChatMessageRoleAssistant = "assistant"
	ChatMessageRoleSystem    = "system"

	ChatMessageStatusPending     = "pending"
	ChatMessageStatusCompleted   = "completed"
	ChatMessageStatusError       = "error"
	ChatMessageStatusInterrupted = "interrupted"
)

var (
	ErrChatUnavailable          = infraerrors.ServiceUnavailable("CHAT_UNAVAILABLE", "Chat history is temporarily unavailable")
	ErrChatConversationNotFound = infraerrors.NotFound("CHAT_CONVERSATION_NOT_FOUND", "Chat conversation not found")
	ErrChatMessageConflict      = infraerrors.Conflict("CHAT_MESSAGE_CONFLICT", "This message has already been submitted")
)

type ChatConversation struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"-"`
	Title           string     `json:"title"`
	DefaultModel    string     `json:"model"`
	APIKeyID        *int64     `json:"api_key_id,omitempty"`
	ReasoningEffort string     `json:"reasoning_effort,omitempty"`
	LastMessageAt   time.Time  `json:"last_message_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"-"`
}

type ChatMessage struct {
	ID               int64           `json:"id"`
	ConversationID   int64           `json:"conversation_id"`
	ClientMessageID  *string         `json:"client_message_id,omitempty"`
	Role             string          `json:"role"`
	Content          string          `json:"content"`
	Model            string          `json:"model,omitempty"`
	Status           string          `json:"status"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	Usage            json.RawMessage `json:"usage,omitempty"`
	GatewayRequestID string          `json:"gateway_request_id,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type PreparedChatCompletion struct {
	Conversation     ChatConversation
	UserMessage      ChatMessage
	AssistantMessage ChatMessage
	History          []ChatMessage
}

type ChatRepository interface {
	Available() bool
	ListConversations(ctx context.Context, userID int64, limit, offset int) ([]ChatConversation, int64, error)
	CreateConversation(ctx context.Context, conversation *ChatConversation) error
	GetConversation(ctx context.Context, userID, conversationID int64) (*ChatConversation, error)
	UpdateConversation(ctx context.Context, conversation *ChatConversation) error
	DeleteConversation(ctx context.Context, userID, conversationID int64) error
	ListMessages(ctx context.Context, userID, conversationID int64, limit int) ([]ChatMessage, error)
	PrepareCompletion(ctx context.Context, userID, conversationID, apiKeyID int64, model, reasoningEffort, clientMessageID, content, title string, historyLimit int) (*PreparedChatCompletion, error)
	CompleteAssistantMessage(ctx context.Context, userID, conversationID, messageID int64, content, model, requestID string, usage json.RawMessage) error
	FailAssistantMessage(ctx context.Context, userID, conversationID, messageID int64, status, errorMessage string) error
}

type ChatService struct {
	repo               ChatRepository
	maxHistoryMessages int
	maxMessageChars    int
}

func NewChatService(repo ChatRepository, maxHistoryMessages, maxMessageChars int) *ChatService {
	if maxHistoryMessages <= 0 || maxHistoryMessages > 500 {
		maxHistoryMessages = 100
	}
	if maxMessageChars <= 0 {
		maxMessageChars = 120000
	}
	return &ChatService{repo: repo, maxHistoryMessages: maxHistoryMessages, maxMessageChars: maxMessageChars}
}

func (s *ChatService) Available() bool { return s != nil && s.repo != nil && s.repo.Available() }

func (s *ChatService) ListConversations(ctx context.Context, userID int64, page, pageSize int) ([]ChatConversation, int64, error) {
	if !s.Available() {
		return nil, 0, ErrChatUnavailable
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	return s.repo.ListConversations(ctx, userID, pageSize, (page-1)*pageSize)
}

func (s *ChatService) CreateConversation(ctx context.Context, userID int64, title, model, reasoningEffort string, apiKeyID *int64) (*ChatConversation, error) {
	if !s.Available() {
		return nil, ErrChatUnavailable
	}
	title = normalizeChatTitle(title)
	reasoningEffort, err := NormalizeChatReasoningEffort(reasoningEffort)
	if err != nil {
		return nil, err
	}
	conversation := &ChatConversation{UserID: userID, Title: title, DefaultModel: strings.TrimSpace(model), APIKeyID: apiKeyID, ReasoningEffort: reasoningEffort}
	if err := s.repo.CreateConversation(ctx, conversation); err != nil {
		return nil, err
	}
	return conversation, nil
}

func (s *ChatService) UpdateConversation(ctx context.Context, userID, id int64, title, model, reasoningEffort *string, apiKeyID *int64) (*ChatConversation, error) {
	if !s.Available() {
		return nil, ErrChatUnavailable
	}
	conversation, err := s.repo.GetConversation(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if title != nil {
		conversation.Title = normalizeChatTitle(*title)
	}
	if model != nil {
		conversation.DefaultModel = strings.TrimSpace(*model)
	}
	if reasoningEffort != nil {
		conversation.ReasoningEffort, err = NormalizeChatReasoningEffort(*reasoningEffort)
		if err != nil {
			return nil, err
		}
	}
	if apiKeyID != nil {
		conversation.APIKeyID = apiKeyID
	}
	if err := s.repo.UpdateConversation(ctx, conversation); err != nil {
		return nil, err
	}
	return conversation, nil
}

func (s *ChatService) DeleteConversation(ctx context.Context, userID, id int64) error {
	if !s.Available() {
		return ErrChatUnavailable
	}
	return s.repo.DeleteConversation(ctx, userID, id)
}

func (s *ChatService) ListMessages(ctx context.Context, userID, conversationID int64) ([]ChatMessage, error) {
	if !s.Available() {
		return nil, ErrChatUnavailable
	}
	return s.repo.ListMessages(ctx, userID, conversationID, s.maxHistoryMessages)
}

func (s *ChatService) PrepareCompletion(ctx context.Context, userID, conversationID, apiKeyID int64, model, reasoningEffort, clientMessageID, content string) (*PreparedChatCompletion, error) {
	if !s.Available() {
		return nil, ErrChatUnavailable
	}
	content = strings.TrimSpace(content)
	model = strings.TrimSpace(model)
	reasoningEffort, err := NormalizeChatReasoningEffort(reasoningEffort)
	if err != nil {
		return nil, err
	}
	clientMessageID = strings.TrimSpace(clientMessageID)
	if content == "" || model == "" || clientMessageID == "" {
		return nil, infraerrors.BadRequest("INVALID_CHAT_MESSAGE", "model, content, and client_message_id are required")
	}
	if len(clientMessageID) > 100 || len(model) > 200 {
		return nil, infraerrors.BadRequest("INVALID_CHAT_MESSAGE", "model or client_message_id is too large")
	}
	if len([]rune(content)) > s.maxMessageChars {
		return nil, infraerrors.BadRequest("CHAT_MESSAGE_TOO_LARGE", "Chat message is too large")
	}
	return s.repo.PrepareCompletion(ctx, userID, conversationID, apiKeyID, model, reasoningEffort, clientMessageID, content, normalizeChatTitle(content), s.maxHistoryMessages)
}

func (s *ChatService) CompleteAssistantMessage(ctx context.Context, prepared *PreparedChatCompletion, content, model, requestID string, usage json.RawMessage) error {
	if prepared == nil {
		return nil
	}
	return s.repo.CompleteAssistantMessage(ctx, prepared.Conversation.UserID, prepared.Conversation.ID, prepared.AssistantMessage.ID, content, model, requestID, usage)
}

func (s *ChatService) FailAssistantMessage(ctx context.Context, prepared *PreparedChatCompletion, status, message string) error {
	if prepared == nil {
		return nil
	}
	if status != ChatMessageStatusInterrupted {
		status = ChatMessageStatusError
	}
	return s.repo.FailAssistantMessage(ctx, prepared.Conversation.UserID, prepared.Conversation.ID, prepared.AssistantMessage.ID, status, truncateChatError(message))
}

func normalizeChatTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "New chat"
	}
	runes := []rune(value)
	if len(runes) > 80 {
		value = string(runes[:80])
	}
	return value
}

// NormalizeChatReasoningEffort canonicalizes the provider effort value stored
// with a conversation. Provider-specific validation happens at completion time.
func NormalizeChatReasoningEffort(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", "", "_", "", " ", "").Replace(value)
	switch value {
	case "", "auto", "default":
		return "", nil
	case "none", "minimal", "low", "medium", "high", "max", "ultra":
		return value, nil
	case "xhigh", "extrahigh":
		return "xhigh", nil
	default:
		return "", infraerrors.BadRequest("INVALID_CHAT_REASONING_EFFORT", "Invalid chat reasoning effort")
	}
}

func truncateChatError(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > 1000 {
		return string(runes[:1000])
	}
	return string(runes)
}
