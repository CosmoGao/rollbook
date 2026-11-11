package backup

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/CosmoGao/rollbook/internal/auth"
	"path/filepath"
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

func (s Service) Submit(ctx context.Context, data []byte, actor auth.Principal) (Pending, error) {
	if !actor.IsUser() && actor.Level == auth.Read {
		return Pending{}, auth.ErrForbidden
	}
	f, p, err := Parse(data)
	if err != nil {
		return Pending{}, err
	}
	if f.Kind == "full" && !actor.IsUser() {
		return Pending{}, ErrInvalid
	}
	if f.Kind != "merge" && !actor.IsUser() {
		return Pending{}, ErrInvalid
	}
	if f.Kind == "merge" {
		p, err = s.MergePreview(ctx, f)
	} else {
		err = Validate(ctx, f)
	}
	if err != nil {
		return Pending{}, err
	}
	var b [16]byte
	if _, err = rand.Read(b[:]); err != nil {
		return Pending{}, err
	}
	proposer := actor.Login
	if !actor.IsUser() {
		proposer = actor.KeyName + " (" + actor.IssuerName + ")"
	}
	item := Pending{ID: hex.EncodeToString(b[:]), Kind: f.Kind, Proposer: proposer, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), State: "pending", Preview: p}
	actorData, _ := json.Marshal(actor)
	_, err = s.DB.ExecContext(ctx, "INSERT INTO pending_imports(id,kind,yaml,proposer,created_at,state,actor_json) VALUES(?,?,?,?,?,'pending',?)", item.ID, item.Kind, data, item.Proposer, item.CreatedAt, string(actorData))
	return item, err
}
func (s Service) List(ctx context.Context) ([]Pending, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,kind,proposer,created_at,state,yaml FROM pending_imports ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	result := []Pending{}
	dataByID := map[string][]byte{}
	for rows.Next() {
		var item Pending
		var data []byte
		if err = rows.Scan(&item.ID, &item.Kind, &item.Proposer, &item.CreatedAt, &item.State, &data); err != nil {
			rows.Close()
			return nil, err
		}
		dataByID[item.ID] = data
		result = append(result, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range result {
		if result[i].State == "pending" {
			f, p, e := Parse(dataByID[result[i].ID])
			if e != nil {
				return nil, e
			}
			result[i].Preview = p
			if f.Kind == "merge" {
				result[i].Preview, e = s.MergePreview(ctx, f)
				if e != nil {
					return nil, e
				}
			}
		}
	}
	return result, nil
}
func (s Service) Review(ctx context.Context, id string, approve bool, reviewer string) error {
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
	var proposer, actorData string
	err := s.DB.QueryRowContext(ctx, "SELECT yaml,proposer,actor_json FROM pending_imports WHERE id=? AND state='pending'", id).Scan(&data, &proposer, &actorData)
	if err != nil {
		return err
	}
	f, _, err := Parse(data)
	if err != nil {
		return err
	}
	if f.Kind == "merge" {
		var actor auth.Principal
		if json.Unmarshal([]byte(actorData), &actor) != nil || (actor.Login == "" && actor.KeyID == "") {
			actor.Login = proposer
		}
		err = s.merge(ctx, id, f, actor, reviewer)
	} else {
		if err = s.saveBeforeRestore(ctx); err != nil {
			return err
		}
		err = s.Restore(ctx, f)
	}
	if err != nil {
		return err
	}
	if f.Kind == "merge" {
		return nil
	}
	_, err = s.DB.ExecContext(ctx, "DELETE FROM pending_imports WHERE id=?", id)
	return err
}
func IsInvalid(err error) bool { return errors.Is(err, ErrInvalid) }

func (s Service) saveBeforeRestore(ctx context.Context) error {
	var seq int
	var name, path string
	rows, err := s.DB.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return err
	}
	for rows.Next() {
		if err = rows.Scan(&seq, &name, &path); err != nil {
			rows.Close()
			return err
		}
		if name == "main" {
			break
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("%w: database path unavailable", ErrInvalid)
	}
	data, err := s.Export(ctx, "full")
	if err != nil {
		return err
	}
	return WriteAtomic(filepath.Join(filepath.Dir(path), "before-restore-"+time.Now().UTC().Format("20060102T150405.000000000")+".yaml"), data)
}
