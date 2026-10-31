package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type IntegrationConfig struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Enabled      bool    `json:"enabled"`
	Config       string  `json:"config"` // raw JSON
	LastSyncedAt *string `json:"last_synced_at"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

const cfgCols = `id, type, enabled, config, last_synced_at, created_at, updated_at`

func scanIntegration(s scanner) (IntegrationConfig, error) {
	var c IntegrationConfig
	var enabled int64
	var lastSynced sql.NullString
	err := s.Scan(&c.ID, &c.Type, &enabled, &c.Config, &lastSynced, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return IntegrationConfig{}, err
	}
	c.Enabled = enabled == 1
	c.LastSyncedAt = nullStr(lastSynced)
	return c, nil
}

type IntegrationConfigStore struct{ db *sql.DB }

func NewIntegrationConfigStore(db *sql.DB) *IntegrationConfigStore {
	return &IntegrationConfigStore{db: db}
}

func (s *IntegrationConfigStore) Get(ctx context.Context, typ string) (IntegrationConfig, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+cfgCols+` FROM integration_configs WHERE type = ?`, typ)
	c, err := scanIntegration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IntegrationConfig{}, ErrNotFound
	}
	return c, err
}

func (s *IntegrationConfigStore) Upsert(ctx context.Context, id, typ, configJSON string) (IntegrationConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO integration_configs (id, type, config)
		VALUES (?,?,?)
		ON CONFLICT(type) DO UPDATE SET
			config     = excluded.config,
			enabled    = 1,
			updated_at = datetime('now')
		RETURNING `+cfgCols,
		id, typ, configJSON,
	)
	return scanIntegration(row)
}

func (s *IntegrationConfigStore) UpdateConfig(ctx context.Context, typ, configJSON string) (IntegrationConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE integration_configs SET config = ?, updated_at = datetime('now')
		WHERE type = ?
		RETURNING `+cfgCols,
		configJSON, typ,
	)
	c, err := scanIntegration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IntegrationConfig{}, ErrNotFound
	}
	return c, err
}

func (s *IntegrationConfigStore) TouchSyncTime(ctx context.Context, typ string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE integration_configs SET last_synced_at = datetime('now') WHERE type = ?`, typ)
	return err
}

func (s *IntegrationConfigStore) SetEnabled(ctx context.Context, typ string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE integration_configs SET enabled=?, updated_at=datetime('now') WHERE type=?`, v, typ)
	return err
}

func (s *IntegrationConfigStore) Delete(ctx context.Context, typ string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM integration_configs WHERE type = ?`, typ)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AITitleType is the integration_configs key for the AI settings. (It began as
// the local-model email-title cleanup and grew into the one model config every
// AI feature uses; the key stays for compatibility.)
const AITitleType = "ai_title"

// Where AI requests go. Mirrors ai.Provider*; kept here so db needn't import ai.
const (
	AIProviderOllama   = "ollama"
	AIProviderAperture = "aperture"
)

// AITitleConfig is the effective AI configuration. The stored config JSON
// carries the fields below; on/off uses the enabled column.
type AITitleConfig struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url"` // Ollama on this host
	Model   string `json:"model"`
	// Provider picks which of the two is used: "" or "ollama" means BaseURL +
	// Model above; "aperture" means a Tailscale Aperture gateway at GatewayURL,
	// asking for GatewayModel. Both sets are kept, so switching back and forth
	// doesn't lose either.
	Provider     string `json:"provider,omitempty"`
	GatewayURL   string `json:"gateway_url,omitempty"`
	GatewayModel string `json:"gateway_model,omitempty"`
}

// Effective returns the provider in use and the URL + model for it.
func (c AITitleConfig) Effective() (provider, baseURL, model string) {
	if c.Provider == AIProviderAperture {
		return AIProviderAperture, c.GatewayURL, c.GatewayModel
	}
	return AIProviderOllama, c.BaseURL, c.Model
}

// AIEnv seeds the AI settings from the environment until the owner saves some.
type AIEnv struct {
	OllamaBaseURL string // OLLAMA_BASE_URL
	OllamaModel   string // OLLAMA_MODEL
	GatewayURL    string // APERTURE_BASE_URL
}

// ResolveAITitle is ResolveAI with only the Ollama env seeds.
func (s *IntegrationConfigStore) ResolveAITitle(ctx context.Context, envBaseURL, envModel string) AITitleConfig {
	return s.ResolveAI(ctx, AIEnv{OllamaBaseURL: envBaseURL, OllamaModel: envModel})
}

// ResolveAI returns the effective settings: the DB-stored config if the owner
// has saved one, otherwise the env defaults (enabled when a model server or a
// gateway URL is configured; the gateway is the default provider only when it
// is the one configured). Empty stored fields fall back to env.
func (s *IntegrationConfigStore) ResolveAI(ctx context.Context, env AIEnv) AITitleConfig {
	cfg := AITitleConfig{
		Enabled:    env.OllamaBaseURL != "" || env.GatewayURL != "",
		BaseURL:    env.OllamaBaseURL,
		Model:      env.OllamaModel,
		GatewayURL: env.GatewayURL,
	}
	if env.OllamaBaseURL == "" && env.GatewayURL != "" {
		cfg.Provider = AIProviderAperture
	}
	rec, err := s.Get(ctx, AITitleType)
	if err != nil {
		return cfg
	}
	cfg.Enabled = rec.Enabled
	var stored AITitleConfig
	if json.Unmarshal([]byte(rec.Config), &stored) == nil {
		if stored.BaseURL != "" {
			cfg.BaseURL = stored.BaseURL
		}
		if stored.Model != "" {
			cfg.Model = stored.Model
		}
		if stored.Provider != "" {
			cfg.Provider = stored.Provider
		}
		if stored.GatewayURL != "" {
			cfg.GatewayURL = stored.GatewayURL
		}
		if stored.GatewayModel != "" {
			cfg.GatewayModel = stored.GatewayModel
		}
	}
	return cfg
}
