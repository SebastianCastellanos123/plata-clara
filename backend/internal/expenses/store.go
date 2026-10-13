package expenses

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

var ErrNotFound = errors.New("gasto no encontrado")

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// REGLA CLAVE (control de acceso): toda consulta incluye "AND user_id = $N".
// Así un usuario NUNCA puede ver, editar ni borrar gastos de otro, aunque
// adivine su id. Si el gasto no es tuyo, para ti simplemente "no existe" (404),
// y no revelamos si existe en la cuenta de otra persona.
// Esto evita la vulnerabilidad #1 del OWASP Top 10: Broken Access Control (IDOR).

const cols = `id, amount, category, description, spent_on`

func scan(row interface{ Scan(...any) error }) (Expense, error) {
	var e Expense
	var d time.Time
	if err := row.Scan(&e.ID, &e.Amount, &e.Category, &e.Description, &d); err != nil {
		return Expense{}, err
	}
	e.SpentOn = d.Format(dateLayout)
	return e, nil
}

func (s *Store) Create(ctx context.Context, userID string, in Input) (Expense, error) {
	return scan(s.db.QueryRowContext(ctx,
		`INSERT INTO expenses (user_id, amount, category, description, spent_on)
		 VALUES ($1, $2, $3, $4, $5::date)
		 RETURNING `+cols,
		userID, in.Amount, in.Category, in.Description, in.SpentOn))
}

func (s *Store) List(ctx context.Context, userID, from, to string) ([]Expense, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+cols+` FROM expenses
		 WHERE user_id = $1 AND spent_on >= $2::date AND spent_on < $3::date
		 ORDER BY spent_on DESC, created_at DESC`,
		userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Expense{} // slice vacío (no nil) para que el JSON sea [] y no null
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) Update(ctx context.Context, userID, id string, in Input) (Expense, error) {
	if !uuidRe.MatchString(id) {
		return Expense{}, ErrNotFound
	}
	e, err := scan(s.db.QueryRowContext(ctx,
		`UPDATE expenses
		 SET amount = $3, category = $4, description = $5, spent_on = $6::date, updated_at = now()
		 WHERE id = $1 AND user_id = $2
		 RETURNING `+cols,
		id, userID, in.Amount, in.Category, in.Description, in.SpentOn))
	if errors.Is(err, sql.ErrNoRows) {
		return Expense{}, ErrNotFound
	}
	return e, err
}

func (s *Store) Delete(ctx context.Context, userID, id string) error {
	if !uuidRe.MatchString(id) {
		return ErrNotFound
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM expenses WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

type CategoryTotal struct {
	Category string `json:"category"`
	Total    int64  `json:"total"`
	Count    int64  `json:"count"`
}

type Summary struct {
	Total      int64           `json:"total"`
	ByCategory []CategoryTotal `json:"byCategory"`
}

func (s *Store) Summary(ctx context.Context, userID, from, to string) (Summary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT category, SUM(amount)::bigint, COUNT(*)
		 FROM expenses
		 WHERE user_id = $1 AND spent_on >= $2::date AND spent_on < $3::date
		 GROUP BY category
		 ORDER BY 2 DESC`,
		userID, from, to)
	if err != nil {
		return Summary{}, err
	}
	defer rows.Close()

	sum := Summary{ByCategory: []CategoryTotal{}}
	for rows.Next() {
		var c CategoryTotal
		if err := rows.Scan(&c.Category, &c.Total, &c.Count); err != nil {
			return Summary{}, err
		}
		sum.Total += c.Total
		sum.ByCategory = append(sum.ByCategory, c)
	}
	return sum, rows.Err()
}
