package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChatRepositoryPrepareCompletionScopesConversationToUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &chatRepository{db: &ChatDB{DB: db, Configured: true}}
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT id, user_id, title, default_model, api_key_id, reasoning_effort, last_message_at, created_at, updated_at
			FROM chat_conversations
			WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(10), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "default_model", "api_key_id", "reasoning_effort", "last_message_at", "created_at", "updated_at"}).
			AddRow(10, 20, "New chat", "", nil, "", now, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM chat_messages WHERE conversation_id = $1 AND client_message_id = $2`)).
		WithArgs(int64(10), "client-1").WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectRollback()

	_, err = repo.PrepareCompletion(context.Background(), 20, 10, 30, "gpt-test", "high", "client-1", "hello", "hello", 100)
	require.Error(t, err)
	require.NotErrorIs(t, err, service.ErrChatConversationNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatRepositoryUpdateConversationPersistsReasoningEffort(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &chatRepository{db: &ChatDB{DB: db, Configured: true}}
	apiKeyID := int64(30)

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE chat_conversations
		SET title = $1, default_model = $2, api_key_id = $3, reasoning_effort = $4, updated_at = NOW()
		WHERE id = $5 AND user_id = $6 AND deleted_at IS NULL`)).
		WithArgs("Reasoning chat", "gpt-test", &apiKeyID, "xhigh", int64(10), int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpdateConversation(context.Background(), &service.ChatConversation{
		ID:              10,
		UserID:          20,
		Title:           "Reasoning chat",
		DefaultModel:    "gpt-test",
		APIKeyID:        &apiKeyID,
		ReasoningEffort: "xhigh",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatRepositoryUnavailable(t *testing.T) {
	repo := NewChatRepository(&ChatDB{})
	_, _, err := repo.ListConversations(context.Background(), 1, 20, 0)
	require.ErrorIs(t, err, service.ErrChatUnavailable)
}
