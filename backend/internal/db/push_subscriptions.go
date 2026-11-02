package db

import (
	"context"
	"database/sql"
	"time"
)

// PushSubscription is a W3C Web Push (VAPID) endpoint registered by a browser /
// PWA. The encryption keys (p256dh, auth) are base64url strings exactly as the
// browser's PushSubscription.toJSON() reports them.
type PushSubscription struct {
	ID        string `json:"id"`
	Endpoint  string `json:"endpoint"`
	P256dh    string `json:"p256dh"`
	Auth      string `json:"auth"`
	Platform  string `json:"platform"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type PushSubStore struct {
	db *sql.DB
}

func NewPushSubStore(database *sql.DB) *PushSubStore {
	return &PushSubStore{db: database}
}

// Upsert registers (or refreshes) a subscription keyed by its endpoint, owned by
// ownerID. Re-subscribing the same browser under another account moves it to
// that account, so a shared device only ever gets its current user's reminders.
func (s *PushSubStore) Upsert(id, endpoint, p256dh, auth, platform, ownerID string) (*PushSubscription, error) {
	now := time.Now().UTC().Format(time.DateTime)
	if platform == "" {
		platform = "web"
	}
	owner, err := resolveOwner(context.Background(), s.db, ownerID)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`
		INSERT INTO push_subscriptions (id, endpoint, p256dh, auth, platform, created_at, updated_at, owner_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
			p256dh = excluded.p256dh,
			auth = excluded.auth,
			platform = excluded.platform,
			owner_id = excluded.owner_id,
			updated_at = excluded.updated_at`,
		id, endpoint, p256dh, auth, platform, now, now, owner)
	if err != nil {
		return nil, err
	}
	return s.GetByEndpoint(endpoint)
}

func (s *PushSubStore) GetByEndpoint(endpoint string) (*PushSubscription, error) {
	row := s.db.QueryRow(
		`SELECT id, endpoint, p256dh, auth, platform, created_at, updated_at
		   FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return scanPushSub(row)
}

func (s *PushSubStore) DeleteByEndpoint(endpoint string) error {
	_, err := s.db.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

// DeleteByEndpointOwned removes ownerID's subscription for endpoint (the API
// path, so a member can only unsubscribe their own browsers).
func (s *PushSubStore) DeleteByEndpointOwned(endpoint, ownerID string) error {
	scope, sargs := ownScope(ownerID)
	_, err := s.db.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`+scope, append([]any{endpoint}, sargs...)...)
	return err
}

// ListForOwner returns the subscriptions registered by ownerID — the only ones
// that user's reminders may be delivered to.
func (s *PushSubStore) ListForOwner(ownerID string) ([]PushSubscription, error) {
	scope, sargs := ownScope(ownerID)
	rows, err := s.db.Query(
		`SELECT id, endpoint, p256dh, auth, platform, created_at, updated_at
		   FROM push_subscriptions WHERE 1=1`+scope+` ORDER BY updated_at DESC`, sargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PushSubscription
	for rows.Next() {
		sub, err := scanPushSub(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sub)
	}
	return out, rows.Err()
}

func (s *PushSubStore) ListAll() ([]PushSubscription, error) {
	rows, err := s.db.Query(
		`SELECT id, endpoint, p256dh, auth, platform, created_at, updated_at
		   FROM push_subscriptions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PushSubscription
	for rows.Next() {
		sub, err := scanPushSub(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sub)
	}
	return out, rows.Err()
}

func scanPushSub(s scanner) (*PushSubscription, error) {
	var p PushSubscription
	if err := s.Scan(&p.ID, &p.Endpoint, &p.P256dh, &p.Auth, &p.Platform, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}
