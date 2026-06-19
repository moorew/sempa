package db

import (
	"crypto/rand"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// PairedDevice is a Dock (or other device) paired to the account. The `token`
// is a normal session id minted on approval, so the existing auth middleware
// validates it; revoking deletes that session.
type PairedDevice struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	DeviceName string  `json:"device_name"`
	Platform   string  `json:"platform"`
	Status     string  `json:"status"`
	Token      *string `json:"-"` // never serialized to clients listing devices
	ExpiresAt  string  `json:"expires_at"`
	CreatedAt  string  `json:"created_at"`
	ApprovedAt *string `json:"approved_at"`
	RevokedAt  *string `json:"revoked_at"`
}

type PairingStore struct {
	db *sql.DB
}

func NewPairingStore(database *sql.DB) *PairingStore {
	return &PairingStore{db: database}
}

// pairingCodeAlphabet excludes ambiguous characters (0/O, 1/I/L) so the code is
// easy to read off a screen and type into the approving app.
const pairingCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func newPairingCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i, v := range b {
		out[i] = pairingCodeAlphabet[int(v)%len(pairingCodeAlphabet)]
	}
	return string(out)
}

// CreatePending starts a pairing: a fresh code valid for `ttl`.
func (s *PairingStore) CreatePending(deviceName, platform string, ttl time.Duration) (*PairedDevice, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	code := newPairingCode()
	_, err := s.db.Exec(`
		INSERT INTO paired_devices (id, code, device_name, platform, status, expires_at, created_at)
		VALUES (?, ?, ?, ?, 'pending', ?, ?)`,
		id, code, deviceName, platform,
		now.Add(ttl).Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return s.GetByCode(code)
}

func (s *PairingStore) GetByCode(code string) (*PairedDevice, error) {
	row := s.db.QueryRow(`
		SELECT id, code, device_name, platform, status, token, expires_at, created_at, approved_at, revoked_at
		FROM paired_devices WHERE code = ?`, code)
	return scanPairedDevice(row)
}

// Approve marks a pending, unexpired code approved and attaches the minted
// session token. Returns ErrNotFound if the code is unknown, expired, or already
// resolved — so the caller should only mint the session once this succeeds.
func (s *PairingStore) Approve(code, token string) (*PairedDevice, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		UPDATE paired_devices
		SET status = 'approved', token = ?, approved_at = ?
		WHERE code = ? AND status = 'pending' AND expires_at > ?`,
		token, now, code, now)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetByCode(code)
}

// ClaimToken returns the device token to the device exactly once (the status
// poll). Subsequent calls return "" so a leaked code can't re-fetch it.
func (s *PairingStore) ClaimToken(code string) (string, error) {
	var token sql.NullString
	row := s.db.QueryRow(`
		SELECT token FROM paired_devices
		WHERE code = ? AND status = 'approved' AND token_claimed = 0`, code)
	if err := row.Scan(&token); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	if _, err := s.db.Exec(`UPDATE paired_devices SET token_claimed = 1 WHERE code = ?`, code); err != nil {
		return "", err
	}
	return token.String, nil
}

// List returns approved, non-revoked devices (for the account's device manager).
func (s *PairingStore) List() ([]PairedDevice, error) {
	rows, err := s.db.Query(`
		SELECT id, code, device_name, platform, status, token, expires_at, created_at, approved_at, revoked_at
		FROM paired_devices WHERE status = 'approved' ORDER BY approved_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PairedDevice{}
	for rows.Next() {
		d, err := scanPairedDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// Revoke flags the device revoked and returns its session token (so the caller
// can delete the session). Returns "" if already revoked / unknown.
func (s *PairingStore) Revoke(id string) (string, error) {
	var token sql.NullString
	row := s.db.QueryRow(`SELECT token FROM paired_devices WHERE id = ? AND status = 'approved'`, id)
	if err := row.Scan(&token); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	_, err := s.db.Exec(`
		UPDATE paired_devices SET status = 'revoked', revoked_at = ?, token = NULL WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id)
	return token.String, err
}

func scanPairedDevice(row scanner) (*PairedDevice, error) {
	var d PairedDevice
	var token, approvedAt, revokedAt sql.NullString
	err := row.Scan(&d.ID, &d.Code, &d.DeviceName, &d.Platform, &d.Status,
		&token, &d.ExpiresAt, &d.CreatedAt, &approvedAt, &revokedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if token.Valid {
		d.Token = &token.String
	}
	if approvedAt.Valid {
		d.ApprovedAt = &approvedAt.String
	}
	if revokedAt.Valid {
		d.RevokedAt = &revokedAt.String
	}
	return &d, nil
}
