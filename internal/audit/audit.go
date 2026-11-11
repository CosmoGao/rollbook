package audit

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
)

type Entry struct {
	ID, ObjectType, ObjectID, Action, Via, Reason, State string
	Diff                                                 any
	BaseRev                                              int64
	Actor                                                auth.Principal
	ReviewedBy, ReviewedAt                               string
}

// AppendTx keeps the actor snapshot with the change, including key issuer details.
func AppendTx(ctx context.Context, tx *sql.Tx, e Entry) (string, error) {
	if e.ObjectID == "" || e.ObjectType == "" || e.Action == "" || e.Reason == "" || e.Via == "" {
		return "", errors.New("audit fields and reason required")
	}
	b, err := json.Marshal(e.Diff)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	e.ID = "change:" + hex.EncodeToString(nonce)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	actor := e.Actor.Login
	kind := "user"
	if e.Actor.KeyID != "" {
		actor = e.Actor.KeyName + " (" + e.Actor.IssuerName + ")"
		kind = "key"
	}
	if actor == "" {
		return "", auth.ErrUnauthorized
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES(?,'change',?,?,?,?,?)`, e.ID, e.Action, now, actor, now, actor)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO changes(id,object_type,object_id,action,diff_json,actor,actor_type,actor_key_id,actor_key_name,actor_issuer_id,actor_issuer_name,via,reason,state,base_rev,reviewed_by,reviewed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, e.ID, e.ObjectType, e.ObjectID, e.Action, string(b), actor, kind, nullable(e.Actor.KeyID), nullable(e.Actor.KeyName), nullable(e.Actor.IssuerID), nullable(e.Actor.IssuerName), e.Via, e.Reason, e.State, e.BaseRev, nullable(e.ReviewedBy), nullable(e.ReviewedAt))
	return e.ID, err
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
