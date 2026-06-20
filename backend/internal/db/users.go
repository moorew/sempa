package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// User is a real account. PasswordHash is nil for Google-only sign-ins.
type User struct {
	ID           string  `json:"id"`
	Email        string  `json:"email"`
	Name         string  `json:"name"`
	IsAdmin      bool    `json:"is_admin"`
	HasPassword  bool    `json:"has_password"` // derived; the hash is never serialized
	PasswordHash *string `json:"-"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

const userCols = `id, email, name, password_hash, is_admin, created_at, updated_at`

func scanUser(s scanner) (User, error) {
	var u User
	var hash sql.NullString
	var isAdmin int64
	if err := s.Scan(&u.ID, &u.Email, &u.Name, &hash, &isAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return User{}, err
	}
	u.PasswordHash = nullStr(hash)
	u.HasPassword = u.PasswordHash != nil && *u.PasswordHash != ""
	u.IsAdmin = isAdmin == 1
	return u, nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// HashPassword returns a bcrypt hash. CheckPassword verifies one.
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// dummyHash is a real bcrypt hash so CheckPasswordDummy does equivalent work to
// a genuine verify — masking account existence via response timing.
var dummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("sempa-timing-dummy"), bcrypt.DefaultCost)
	return string(h)
}()

// CheckPasswordDummy performs a constant-work compare for the "no such user"
// path so login timing doesn't reveal whether an account exists.
func CheckPasswordDummy(pw string) { _ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(pw)) }

type UserStore struct{ db *sql.DB }

func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db: db} }

var ErrUserNotFound = errors.New("user not found")

func (s *UserStore) GetByEmail(ctx context.Context, email string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE email = ?`, normalizeEmail(email)))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return u, err
}

func (s *UserStore) GetByID(ctx context.Context, id string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return u, err
}

func (s *UserStore) List(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// PrimaryID returns the oldest user's id — the household's primary account that
// owns all pre-multi-user data. Empty string if there are no users.
func (s *UserStore) PrimaryID(ctx context.Context) (string, error) {
	return PrimaryUserID(ctx, s.db)
}

func (s *UserStore) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *UserStore) CountWithPassword(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE password_hash IS NOT NULL AND password_hash != ''`).Scan(&n)
	return n, err
}

type CreateUserParams struct {
	Email        string
	Name         string
	PasswordHash *string
	IsAdmin      bool
}

func (s *UserStore) Create(ctx context.Context, p CreateUserParams) (User, error) {
	isAdmin := 0
	if p.IsAdmin {
		isAdmin = 1
	}
	return scanUser(s.db.QueryRowContext(ctx, `
		INSERT INTO users (id, email, name, password_hash, is_admin) VALUES (?,?,?,?,?)
		RETURNING `+userCols,
		uuid.New().String(), normalizeEmail(p.Email), p.Name, p.PasswordHash, isAdmin))
}

// EnsureByEmail returns the user with this email, creating a password-less one
// (e.g. a Google sign-in) if absent. isAdmin only applies on creation.
func (s *UserStore) EnsureByEmail(ctx context.Context, email, name string, isAdmin bool) (User, error) {
	if u, err := s.GetByEmail(ctx, email); err == nil {
		return u, nil
	} else if !errors.Is(err, ErrUserNotFound) {
		return User{}, err
	}
	return s.Create(ctx, CreateUserParams{Email: email, Name: name, IsAdmin: isAdmin})
}

func (s *UserStore) SetPassword(ctx context.Context, id, hash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = datetime('now') WHERE id = ?`, hash, id)
	return err
}

func (s *UserStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}
