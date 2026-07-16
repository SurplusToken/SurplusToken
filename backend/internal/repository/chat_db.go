package repository

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/Wei-Shaw/sub2api/chatmigrations"
	"github.com/Wei-Shaw/sub2api/internal/config"
)

// ChatDB is a distinct database handle so Wire cannot confuse it with the
// primary *sql.DB. Chat is optional: connection failures disable only the chat
// endpoints and never prevent the gateway from starting.
type ChatDB struct {
	DB         *sql.DB
	Configured bool
}

func (d *ChatDB) Available() bool { return d != nil && d.DB != nil }

func (d *ChatDB) Close() error {
	if d == nil || d.DB == nil {
		return nil
	}
	return d.DB.Close()
}

func ProvideChatDB(cfg *config.Config) *ChatDB {
	result := &ChatDB{Configured: cfg != nil && cfg.Chat.Enabled}
	if !result.Configured {
		return result
	}

	db, err := sql.Open("postgres", cfg.Chat.Database.DSNWithTimezone(cfg.Timezone))
	if err != nil {
		log.Printf("Warning: chat database disabled: open failed: %v", err)
		return result
	}
	applyChatDBPoolSettings(db, cfg.Chat.Database)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		log.Printf("Warning: chat database disabled: ping failed: %v", err)
		return result
	}
	if err := applyMigrationsFS(ctx, db, chatmigrations.FS); err != nil {
		_ = db.Close()
		log.Printf("Warning: chat database disabled: migration failed: %v", err)
		return result
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE chat_messages
		SET status = 'interrupted', error_message = 'Generation was interrupted', updated_at = NOW()
		WHERE status = 'pending' AND updated_at < NOW() - INTERVAL '5 minutes'
	`); err != nil {
		_ = db.Close()
		log.Printf("Warning: chat database disabled: recovery failed: %v", err)
		return result
	}

	result.DB = db
	return result
}

func applyChatDBPoolSettings(db *sql.DB, cfg config.DatabaseConfig) {
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 32
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle < 0 {
		maxIdle = 0
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	if cfg.ConnMaxLifetimeMinutes > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMinutes) * time.Minute)
	}
	if cfg.ConnMaxIdleTimeMinutes > 0 {
		db.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTimeMinutes) * time.Minute)
	}
}
