package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type chatRepository struct {
	db *ChatDB
}

func NewChatRepository(db *ChatDB) service.ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) Available() bool { return r != nil && r.db != nil && r.db.Available() }

func (r *chatRepository) sqlDB() (*sql.DB, error) {
	if !r.Available() {
		return nil, service.ErrChatUnavailable
	}
	return r.db.DB, nil
}

func (r *chatRepository) ListConversations(ctx context.Context, userID int64, limit, offset int) ([]service.ChatConversation, int64, error) {
	db, err := r.sqlDB()
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_conversations WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count chat conversations: %w", err)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, title, default_model, api_key_id, reasoning_effort, last_message_at, created_at, updated_at
		FROM chat_conversations
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY updated_at DESC, id DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list chat conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ChatConversation, 0)
	for rows.Next() {
		conversation, scanErr := scanChatConversation(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, conversation)
	}
	return items, total, rows.Err()
}

func (r *chatRepository) CreateConversation(ctx context.Context, c *service.ChatConversation) error {
	db, err := r.sqlDB()
	if err != nil {
		return err
	}
	return db.QueryRowContext(ctx, `
		INSERT INTO chat_conversations (user_id, title, default_model, api_key_id, reasoning_effort)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, last_message_at, created_at, updated_at`, c.UserID, c.Title, c.DefaultModel, c.APIKeyID, c.ReasoningEffort).
		Scan(&c.ID, &c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt)
}

func (r *chatRepository) GetConversation(ctx context.Context, userID, conversationID int64) (*service.ChatConversation, error) {
	db, err := r.sqlDB()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `
		SELECT id, user_id, title, default_model, api_key_id, reasoning_effort, last_message_at, created_at, updated_at
		FROM chat_conversations WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, conversationID, userID)
	conversation, err := scanChatConversation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrChatConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get chat conversation: %w", err)
	}
	return &conversation, nil
}

func (r *chatRepository) UpdateConversation(ctx context.Context, c *service.ChatConversation) error {
	db, err := r.sqlDB()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `
		UPDATE chat_conversations
		SET title = $1, default_model = $2, api_key_id = $3, reasoning_effort = $4, updated_at = NOW()
		WHERE id = $5 AND user_id = $6 AND deleted_at IS NULL`, c.Title, c.DefaultModel, c.APIKeyID, c.ReasoningEffort, c.ID, c.UserID)
	if err != nil {
		return fmt.Errorf("update chat conversation: %w", err)
	}
	return requireChatAffected(result)
}

func (r *chatRepository) DeleteConversation(ctx context.Context, userID, conversationID int64) error {
	db, err := r.sqlDB()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `
		UPDATE chat_conversations SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, conversationID, userID)
	if err != nil {
		return fmt.Errorf("delete chat conversation: %w", err)
	}
	return requireChatAffected(result)
}

func (r *chatRepository) ListMessages(ctx context.Context, userID, conversationID int64, limit int) ([]service.ChatMessage, error) {
	db, err := r.sqlDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, conversation_id, client_message_id, role, content, model, status,
		       error_message, usage, gateway_request_id, created_at, updated_at
		FROM (
			SELECT m.* FROM chat_messages m
			JOIN chat_conversations c ON c.id = m.conversation_id
			WHERE m.conversation_id = $1 AND c.user_id = $2 AND c.deleted_at IS NULL
			ORDER BY m.created_at DESC, m.id DESC LIMIT $3
		) recent
		ORDER BY created_at ASC, id ASC`, conversationID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ChatMessage, 0)
	for rows.Next() {
		message, scanErr := scanChatMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		if _, err := r.GetConversation(ctx, userID, conversationID); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (r *chatRepository) PrepareCompletion(ctx context.Context, userID, conversationID, apiKeyID int64, model, reasoningEffort, clientMessageID, content, title string, historyLimit int) (*service.PreparedChatCompletion, error) {
	db, err := r.sqlDB()
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	conversation, err := scanChatConversation(tx.QueryRowContext(ctx, `
		SELECT id, user_id, title, default_model, api_key_id, reasoning_effort, last_message_at, created_at, updated_at
		FROM chat_conversations
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL FOR UPDATE`, conversationID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrChatConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock chat conversation: %w", err)
	}

	var duplicateID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM chat_messages WHERE conversation_id = $1 AND client_message_id = $2`, conversationID, clientMessageID).Scan(&duplicateID)
	if err == nil {
		return nil, service.ErrChatMessageConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check chat message idempotency: %w", err)
	}

	prepared := &service.PreparedChatCompletion{Conversation: conversation}
	prepared.Conversation.DefaultModel = model
	prepared.Conversation.APIKeyID = &apiKeyID
	prepared.Conversation.ReasoningEffort = reasoningEffort
	err = tx.QueryRowContext(ctx, `
		INSERT INTO chat_messages (conversation_id, client_message_id, role, content, model, status)
		VALUES ($1, $2, 'user', $3, $4, 'completed')
		RETURNING id, created_at, updated_at`, conversationID, clientMessageID, content, model).
		Scan(&prepared.UserMessage.ID, &prepared.UserMessage.CreatedAt, &prepared.UserMessage.UpdatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, service.ErrChatMessageConflict
		}
		return nil, fmt.Errorf("insert chat user message: %w", err)
	}
	prepared.UserMessage.ConversationID = conversationID
	prepared.UserMessage.ClientMessageID = &clientMessageID
	prepared.UserMessage.Role = service.ChatMessageRoleUser
	prepared.UserMessage.Content = content
	prepared.UserMessage.Model = model
	prepared.UserMessage.Status = service.ChatMessageStatusCompleted

	err = tx.QueryRowContext(ctx, `
		INSERT INTO chat_messages (conversation_id, role, model, status)
		VALUES ($1, 'assistant', $2, 'pending')
		RETURNING id, created_at, updated_at`, conversationID, model).
		Scan(&prepared.AssistantMessage.ID, &prepared.AssistantMessage.CreatedAt, &prepared.AssistantMessage.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert pending assistant message: %w", err)
	}
	prepared.AssistantMessage.ConversationID = conversationID
	prepared.AssistantMessage.Role = service.ChatMessageRoleAssistant
	prepared.AssistantMessage.Model = model
	prepared.AssistantMessage.Status = service.ChatMessageStatusPending

	newTitle := conversation.Title
	if strings.EqualFold(strings.TrimSpace(newTitle), "new chat") || strings.TrimSpace(newTitle) == "新对话" {
		newTitle = title
	}
	err = tx.QueryRowContext(ctx, `
		UPDATE chat_conversations
		SET title = $1, default_model = $2, api_key_id = $3, reasoning_effort = $4, last_message_at = NOW(), updated_at = NOW()
		WHERE id = $5
		RETURNING title, last_message_at, updated_at`, newTitle, model, apiKeyID, reasoningEffort, conversationID).
		Scan(&prepared.Conversation.Title, &prepared.Conversation.LastMessageAt, &prepared.Conversation.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("touch chat conversation: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, conversation_id, client_message_id, role, content, model, status,
		       error_message, usage, gateway_request_id, created_at, updated_at
		FROM (
			SELECT * FROM chat_messages
			WHERE conversation_id = $1 AND id <> $2
			  AND status = 'completed' AND role IN ('user', 'assistant', 'system')
			ORDER BY created_at DESC, id DESC LIMIT $3
		) history
		ORDER BY created_at ASC, id ASC`, conversationID, prepared.AssistantMessage.ID, historyLimit)
	if err != nil {
		return nil, fmt.Errorf("load chat completion history: %w", err)
	}
	for rows.Next() {
		message, scanErr := scanChatMessage(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		prepared.History = append(prepared.History, message)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit chat completion preparation: %w", err)
	}
	return prepared, nil
}

func (r *chatRepository) CompleteAssistantMessage(ctx context.Context, userID, conversationID, messageID int64, content, model, requestID string, usage json.RawMessage) error {
	db, err := r.sqlDB()
	if err != nil {
		return err
	}
	var usageValue any
	if len(usage) > 0 && json.Valid(usage) {
		usageValue = string(usage)
	}
	result, err := db.ExecContext(ctx, `
		UPDATE chat_messages m SET content = $1, model = $2, status = 'completed', error_message = '',
		       usage = $3, gateway_request_id = $4, updated_at = NOW()
		FROM chat_conversations c
		WHERE m.id = $5 AND m.conversation_id = $6 AND c.id = m.conversation_id
		  AND c.user_id = $7 AND c.deleted_at IS NULL`, content, model, usageValue, requestID, messageID, conversationID, userID)
	if err != nil {
		return fmt.Errorf("complete assistant message: %w", err)
	}
	return requireChatAffected(result)
}

func (r *chatRepository) FailAssistantMessage(ctx context.Context, userID, conversationID, messageID int64, status, errorMessage string) error {
	db, err := r.sqlDB()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `
		UPDATE chat_messages m SET status = $1, error_message = $2, updated_at = NOW()
		FROM chat_conversations c
		WHERE m.id = $3 AND m.conversation_id = $4 AND c.id = m.conversation_id
		  AND c.user_id = $5 AND c.deleted_at IS NULL`, status, errorMessage, messageID, conversationID, userID)
	if err != nil {
		return fmt.Errorf("fail assistant message: %w", err)
	}
	return requireChatAffected(result)
}

type chatScanner interface {
	Scan(dest ...any) error
}

func scanChatConversation(scanner chatScanner) (service.ChatConversation, error) {
	var c service.ChatConversation
	var apiKeyID sql.NullInt64
	err := scanner.Scan(&c.ID, &c.UserID, &c.Title, &c.DefaultModel, &apiKeyID, &c.ReasoningEffort, &c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt)
	if apiKeyID.Valid {
		c.APIKeyID = &apiKeyID.Int64
	}
	return c, err
}

func scanChatMessage(scanner chatScanner) (service.ChatMessage, error) {
	var m service.ChatMessage
	var clientID sql.NullString
	var usage []byte
	err := scanner.Scan(&m.ID, &m.ConversationID, &clientID, &m.Role, &m.Content, &m.Model, &m.Status,
		&m.ErrorMessage, &usage, &m.GatewayRequestID, &m.CreatedAt, &m.UpdatedAt)
	if clientID.Valid {
		m.ClientMessageID = &clientID.String
	}
	if len(usage) > 0 {
		m.Usage = append(json.RawMessage(nil), usage...)
	}
	return m, err
}

func requireChatAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return service.ErrChatConversationNotFound
	}
	return nil
}
