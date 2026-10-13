// Package users contiene el acceso a la tabla users.
package users

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

var (
	ErrNotFound   = errors.New("usuario no encontrado")
	ErrEmailTaken = errors.New("el correo ya está registrado")
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Create inserta un usuario. Observa que TODAS las consultas usan $1, $2...
// (parámetros): el valor viaja separado del SQL, así que nunca puede
// interpretarse como código. Eso es lo que evita la inyección SQL.
func (s *Store) Create(ctx context.Context, email, passwordHash string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2)
		 RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
		return User{}, ErrEmailTaken
	}
	return u, err
}

func (s *Store) ByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) ByID(ctx context.Context, id string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
