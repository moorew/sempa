package db

import (
	"context"
	"database/sql"
	"time"
)

type DeviceToken struct {
	ID        string `json:"id"`
	Token     string `json:"token"`
	Platform  string `json:"platform"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type DeviceTokenStore struct {
	db *sql.DB
}

func NewDeviceTokenStore(database *sql.DB) *DeviceTokenStore {
	return &DeviceTokenStore{db: database}
}

// Upsert registers (or refreshes) an FCM token owned by ownerID. A token
// re-registered under another account moves to it (one phone, one current user).
func (s *DeviceTokenStore) Upsert(id, token, platform, ownerID string) (*DeviceToken, error) {
	now := time.Now().UTC().Format(time.DateTime)
	owner, err := resolveOwner(context.Background(), s.db, ownerID)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`
		INSERT INTO device_tokens (id, token, platform, created_at, updated_at, owner_id)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(token) DO UPDATE SET updated_at = ?, owner_id = excluded.owner_id`,
		id, token, platform, now, now, owner, now)
	if err != nil {
		return nil, err
	}
	return s.GetByToken(token)
}

func (s *DeviceTokenStore) GetByToken(token string) (*DeviceToken, error) {
	row := s.db.QueryRow(`SELECT id, token, platform, created_at, updated_at FROM device_tokens WHERE token = ?`, token)
	var d DeviceToken
	if err := row.Scan(&d.ID, &d.Token, &d.Platform, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (s *DeviceTokenStore) Delete(token string) error {
	_, err := s.db.Exec(`DELETE FROM device_tokens WHERE token = ?`, token)
	return err
}

// DeleteOwned removes ownerID's registration of token (the API path).
func (s *DeviceTokenStore) DeleteOwned(token, ownerID string) error {
	scope, sargs := ownScope(ownerID)
	_, err := s.db.Exec(`DELETE FROM device_tokens WHERE token = ?`+scope, append([]any{token}, sargs...)...)
	return err
}

// ListForOwner returns the FCM tokens registered by ownerID.
func (s *DeviceTokenStore) ListForOwner(ownerID string) ([]DeviceToken, error) {
	scope, sargs := ownScope(ownerID)
	rows, err := s.db.Query(`SELECT id, token, platform, created_at, updated_at FROM device_tokens WHERE 1=1`+scope+` ORDER BY updated_at DESC`, sargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceToken
	for rows.Next() {
		var d DeviceToken
		if err := rows.Scan(&d.ID, &d.Token, &d.Platform, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *DeviceTokenStore) ListAll() ([]DeviceToken, error) {
	rows, err := s.db.Query(`SELECT id, token, platform, created_at, updated_at FROM device_tokens ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceToken
	for rows.Next() {
		var d DeviceToken
		if err := rows.Scan(&d.ID, &d.Token, &d.Platform, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
