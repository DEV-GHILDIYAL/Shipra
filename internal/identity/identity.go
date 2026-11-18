package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleAdmin    = "admin"
	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 1
)

var ErrDenied = errors.New("access denied")

type Principal struct {
	UserID      string `json:"id"`
	Username    string `json:"username"`
	WorkspaceID string `json:"-"`
	Role        string `json:"role"`
	Scope       string `json:"-"`
	SessionID   string `json:"-"`
	CSRFHash    []byte `json:"-"`
}

func Allows(role, scope, action string) bool {
	if action == "read" {
		return role == RoleViewer || role == RoleOperator || role == RoleAdmin
	}
	if action == "write" {
		return (role == RoleOperator || role == RoleAdmin) && scope != "read"
	}
	if action == "admin" {
		return role == RoleAdmin && scope != "read" && scope != "write"
	}
	return false
}

func RandomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func RandomSecret() (string, []byte, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", nil, err
	}
	secret := base64.RawURLEncoding.EncodeToString(b[:])
	hash := sha256.Sum256([]byte(secret))
	return secret, hash[:], nil
}

func Digest(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 1024 {
		return "", errors.New("password must be 12-1024 bytes")
	}
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt[:], argonTime, argonMemory, argonThreads, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, base64.RawStdEncoding.EncodeToString(salt[:]), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=1" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func NormalizeUsername(input string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	if len(s) < 3 || len(s) > 64 {
		return "", errors.New("username must be 3-64 ASCII characters")
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return "", errors.New("username must use letters, digits, dot, hyphen or underscore")
		}
	}
	return s, nil
}

func Audit(ctx context.Context, tx *sql.Tx, workspaceID, actorID, action, target, outcome, requestID string) error {
	id, err := RandomID()
	if err != nil {
		return err
	}
	var actor any
	if actorID != "" {
		actor = actorID
	}
	var workspace any
	if workspaceID != "" {
		workspace = workspaceID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,workspace_id,actor_id,action,target,outcome,request_id) VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, workspace, actor, action, target, outcome, requestID)
	return err
}

func BootstrapAdmin(ctx context.Context, db *sql.DB, username, password string) error {
	name, err := NormalizeUsername(username)
	if err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	workspaceID, err := RandomID()
	if err != nil {
		return err
	}
	userID, err := RandomID()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errors.New("administrator already bootstrapped")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO workspaces(id,name) VALUES ($1,'Default')`, workspaceID); err != nil {
		return errors.New("administrator already bootstrapped")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO users(id,username,password_hash) VALUES ($1,$2,$3)`, userID, name, hash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memberships(workspace_id,user_id,role) VALUES ($1,$2,'admin')`, workspaceID, userID); err != nil {
		return err
	}
	if err := Audit(ctx, tx, workspaceID, userID, "admin.bootstrap", userID, "success", ""); err != nil {
		return err
	}
	return tx.Commit()
}

func AuthenticatePassword(ctx context.Context, db *sql.DB, username, password string) (Principal, error) {
	name, err := NormalizeUsername(username)
	if err != nil {
		return Principal{}, ErrDenied
	}
	var p Principal
	var hash string
	err = db.QueryRowContext(ctx, `SELECT u.id,u.username,u.password_hash,m.workspace_id,m.role FROM users u JOIN memberships m ON m.user_id=u.id WHERE u.username=$1 AND u.active`, name).Scan(&p.UserID, &p.Username, &hash, &p.WorkspaceID, &p.Role)
	if err != nil {
		return Principal{}, ErrDenied
	}
	if !VerifyPassword(hash, password) {
		return Principal{}, ErrDenied
	}
	p.Scope = "admin"
	return p, nil
}

func CreateSession(ctx context.Context, db *sql.DB, p Principal, requestID string) (string, string, error) {
	secret, digest, err := RandomSecret()
	if err != nil {
		return "", "", err
	}
	csrf, csrfDigest, err := RandomSecret()
	if err != nil {
		return "", "", err
	}
	id, err := RandomID()
	if err != nil {
		return "", "", err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO sessions(id,user_id,token_hash,csrf_hash,expires_at) VALUES ($1,$2,$3,$4,now()+interval '12 hours')`, id, p.UserID, digest, csrfDigest)
	if err != nil {
		return "", "", err
	}
	if err := Audit(ctx, tx, p.WorkspaceID, p.UserID, "session.create", id, "success", requestID); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return secret, csrf, nil
}

func Session(ctx context.Context, db *sql.DB, secret string) (Principal, error) {
	if len(secret) != 43 {
		return Principal{}, ErrDenied
	}
	var p Principal
	err := db.QueryRowContext(ctx, `UPDATE sessions s SET last_seen_at=now() FROM users u JOIN memberships m ON m.user_id=u.id WHERE s.user_id=u.id AND s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND s.last_seen_at>now()-interval '30 minutes' AND u.active RETURNING u.id,u.username,m.workspace_id,m.role,s.id,s.csrf_hash`, Digest(secret)).Scan(&p.UserID, &p.Username, &p.WorkspaceID, &p.Role, &p.SessionID, &p.CSRFHash)
	if err != nil {
		return Principal{}, ErrDenied
	}
	p.Scope = "admin"
	return p, nil
}

func RevokeSession(ctx context.Context, db *sql.DB, p Principal, requestID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, p.SessionID); err != nil {
		return err
	}
	if err := Audit(ctx, tx, p.WorkspaceID, p.UserID, "session.revoke", p.SessionID, "success", requestID); err != nil {
		return err
	}
	return tx.Commit()
}

func Token(ctx context.Context, db *sql.DB, secret string) (Principal, error) {
	if len(secret) != 43 {
		return Principal{}, ErrDenied
	}
	var p Principal
	err := db.QueryRowContext(ctx, `SELECT u.id,u.username,m.workspace_id,m.role,t.scope FROM api_tokens t JOIN users u ON u.id=t.user_id JOIN memberships m ON m.user_id=u.id WHERE t.token_hash=$1 AND t.expires_at>now() AND t.revoked_at IS NULL AND u.active`, Digest(secret)).Scan(&p.UserID, &p.Username, &p.WorkspaceID, &p.Role, &p.Scope)
	if err != nil {
		return Principal{}, ErrDenied
	}
	return p, nil
}

func CreateToken(ctx context.Context, db *sql.DB, p Principal, scope string, requestID string) (string, string, error) {
	if scope != "read" && scope != "write" && scope != "admin" {
		return "", "", ErrDenied
	}
	if !Allows(p.Role, p.Scope, scope) {
		return "", "", ErrDenied
	}
	secret, digest, err := RandomSecret()
	if err != nil {
		return "", "", err
	}
	id, err := RandomID()
	if err != nil {
		return "", "", err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO api_tokens(id,user_id,token_hash,scope,expires_at) VALUES ($1,$2,$3,$4,now()+interval '30 days')`, id, p.UserID, digest, scope); err != nil {
		return "", "", err
	}
	if err := Audit(ctx, tx, p.WorkspaceID, p.UserID, "token.create", id, "success", requestID); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return id, secret, nil
}

func RevokeToken(ctx context.Context, db *sql.DB, p Principal, tokenID, requestID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=now() WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL`, tokenID, p.UserID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrDenied
	}
	if err := Audit(ctx, tx, p.WorkspaceID, p.UserID, "token.revoke", tokenID, "success", requestID); err != nil {
		return err
	}
	return tx.Commit()
}

func CSRFMatches(p Principal, token string) bool {
	if p.SessionID == "" || len(token) != 43 {
		return false
	}
	return subtle.ConstantTimeCompare(p.CSRFHash, Digest(token)) == 1
}

func SessionExpiresAt() time.Duration { return 12 * time.Hour }
