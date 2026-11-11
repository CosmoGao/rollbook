package backup

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

type Pending struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Proposer  string  `json:"proposer"`
	CreatedAt string  `json:"created_at"`
	State     string  `json:"state"`
	Preview   Preview `json:"preview"`
}

func (s Service) Submit(ctx context.Context, data []byte, proposer string, allowFull bool) (Pending, error) {
	f, p, err := Parse(data)
	if err != nil {
		return Pending{}, err
	}
	if f.Kind == "full" && !allowFull {
		return Pending{}, ErrInvalid
	}
	if err = Validate(ctx, f); err != nil {
		return Pending{}, err
	}
	var b [16]byte
	if _, err = rand.Read(b[:]); err != nil {
		return Pending{}, err
	}
	item := Pending{ID: hex.EncodeToString(b[:]), Kind: f.Kind, Proposer: proposer, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), State: "pending", Preview: p}
	_, err = s.DB.ExecContext(ctx, "INSERT INTO pending_imports(id,kind,yaml,proposer,created_at,state) VALUES(?,?,?,?,?,'pending')", item.ID, item.Kind, data, item.Proposer, item.CreatedAt)
	return item, err
}
func (s Service) List(ctx context.Context) ([]Pending, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,kind,proposer,created_at,state,yaml FROM pending_imports ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Pending{}
	for rows.Next() {
		var item Pending
		var data []byte
		if err = rows.Scan(&item.ID, &item.Kind, &item.Proposer, &item.CreatedAt, &item.State, &data); err != nil {
			return nil, err
		}
		if item.State == "pending" {
			_, item.Preview, _ = Parse(data)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s Service) Review(ctx context.Context, id string, approve bool) error {
	if !approve {
		r, err := s.DB.ExecContext(ctx, "UPDATE pending_imports SET state='rejected',yaml=x'' WHERE id=? AND state='pending'", id)
		if err != nil {
			return err
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			return sql.ErrNoRows
		}
		return nil
	}
	var data []byte
	err := s.DB.QueryRowContext(ctx, "SELECT yaml FROM pending_imports WHERE id=? AND state='pending'", id).Scan(&data)
	if err != nil {
		return err
	}
	f, _, err := Parse(data)
	if err != nil {
		return err
	}
	if err = s.Restore(ctx, f); err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, "DELETE FROM pending_imports WHERE id=?", id)
	return err
}
func IsInvalid(err error) bool { return errors.Is(err, ErrInvalid) }
