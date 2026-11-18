package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/DEV-GHILDIYAL/Shipra/internal/identity"
)

func validName(s string) bool {
	return len(s) >= 1 && len(s) <= 100 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\r\n\x00")
}
func validRole(s string) bool {
	return s == identity.RoleViewer || s == identity.RoleOperator || s == identity.RoleAdmin
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "read")
	if !ok {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,name,created_at FROM projects WHERE workspace_id=$1 ORDER BY created_at DESC,id DESC LIMIT 200`, p.WorkspaceID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, name string
		var created any
		if err := rows.Scan(&id, &name, &created); err != nil {
			s.internal(w, r, err)
			return
		}
		items = append(items, map[string]any{"id": id, "name": name, "created_at": created})
	}
	if err := rows.Err(); err != nil {
		s.internal(w, r, err)
		return
	}
	respond(w, 200, map[string]any{"projects": items})
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "admin")
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !validName(input.Name) {
		fail(w, r, 422, "invalid_name", "Project name must be 1-100 characters")
		return
	}
	id, err := identity.RandomID()
	if err != nil {
		s.internal(w, r, err)
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO projects(id,workspace_id,name) VALUES ($1,$2,$3)`, id, p.WorkspaceID, input.Name); err != nil {
		fail(w, r, 409, "conflict", "Project name already exists")
		return
	}
	if err := identity.Audit(r.Context(), tx, p.WorkspaceID, p.UserID, "project.create", id, "success", requestID(r)); err != nil {
		s.internal(w, r, err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.internal(w, r, err)
		return
	}
	respond(w, 201, map[string]string{"id": id, "name": input.Name})
}

func (s *Server) listRepositories(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "read")
	if !ok {
		return
	}
	projectID := r.PathValue("id")
	var exists bool
	if err := s.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND workspace_id=$2)`, projectID, p.WorkspaceID).Scan(&exists); err != nil || !exists {
		fail(w, r, 404, "not_found", "Project not found")
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,provider,installation_id,repository_id,default_ref FROM repositories WHERE project_id=$1 AND workspace_id=$2 ORDER BY id LIMIT 200`, projectID, p.WorkspaceID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]string, 0)
	for rows.Next() {
		var id, provider, installationID, repositoryID, ref string
		if err := rows.Scan(&id, &provider, &installationID, &repositoryID, &ref); err != nil {
			s.internal(w, r, err)
			return
		}
		items = append(items, map[string]string{"id": id, "provider": provider, "installation_id": installationID, "repository_id": repositoryID, "default_ref": ref})
	}
	if err := rows.Err(); err != nil {
		s.internal(w, r, err)
		return
	}
	respond(w, 200, map[string]any{"repositories": items})
}

func (s *Server) createRepository(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "admin")
	if !ok {
		return
	}
	var input struct {
		InstallationID string `json:"installation_id"`
		RepositoryID   string `json:"repository_id"`
		DefaultRef     string `json:"default_ref"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !validName(input.InstallationID) || !validName(input.RepositoryID) || !validName(input.DefaultRef) {
		fail(w, r, 422, "invalid_repository", "Invalid repository identity or ref")
		return
	}
	id, err := identity.RandomID()
	if err != nil {
		s.internal(w, r, err)
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `INSERT INTO repositories(id,project_id,workspace_id,provider,installation_id,repository_id,default_ref) SELECT $1,id,workspace_id,'github',$2,$3,$4 FROM projects WHERE id=$5 AND workspace_id=$6`, id, input.InstallationID, input.RepositoryID, input.DefaultRef, r.PathValue("id"), p.WorkspaceID)
	if err != nil {
		fail(w, r, 409, "conflict", "Repository identity already exists")
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		s.internal(w, r, err)
		return
	}
	if n == 0 {
		fail(w, r, 404, "not_found", "Project not found")
		return
	}
	if err := identity.Audit(r.Context(), tx, p.WorkspaceID, p.UserID, "repository.create", id, "success", requestID(r)); err != nil {
		s.internal(w, r, err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.internal(w, r, err)
		return
	}
	respond(w, 201, map[string]string{"id": id})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "admin")
	if !ok {
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	name, err := identity.NormalizeUsername(input.Username)
	if err != nil || !validRole(input.Role) {
		fail(w, r, 422, "invalid_user", "Invalid username or role")
		return
	}
	hash, err := identity.HashPassword(input.Password)
	if err != nil {
		fail(w, r, 422, "invalid_password", err.Error())
		return
	}
	id, err := identity.RandomID()
	if err != nil {
		s.internal(w, r, err)
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO users(id,username,password_hash) VALUES ($1,$2,$3)`, id, name, hash); err != nil {
		fail(w, r, 409, "conflict", "Username already exists")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO memberships(workspace_id,user_id,role) VALUES ($1,$2,$3)`, p.WorkspaceID, id, input.Role); err != nil {
		s.internal(w, r, err)
		return
	}
	if err := identity.Audit(r.Context(), tx, p.WorkspaceID, p.UserID, "user.create", id, "success", requestID(r)); err != nil {
		s.internal(w, r, err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.internal(w, r, err)
		return
	}
	respond(w, 201, map[string]string{"id": id, "username": name, "role": input.Role})
}

func (s *Server) changeRole(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "admin")
	if !ok {
		return
	}
	var input struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !validRole(input.Role) {
		fail(w, r, 422, "invalid_role", "Invalid role")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	defer tx.Rollback()
	var locked string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM workspaces WHERE id=$1 FOR UPDATE`, p.WorkspaceID).Scan(&locked); err != nil {
		s.internal(w, r, err)
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE memberships SET role=$1 WHERE workspace_id=$2 AND user_id=$3`, input.Role, p.WorkspaceID, r.PathValue("id"))
	if err != nil {
		s.internal(w, r, err)
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		s.internal(w, r, err)
		return
	}
	if n == 0 {
		fail(w, r, 404, "not_found", "User not found")
		return
	}
	var admins int
	if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.role='admin' AND u.active`, p.WorkspaceID).Scan(&admins); err != nil {
		s.internal(w, r, err)
		return
	}
	if admins == 0 {
		fail(w, r, 409, "last_admin", "At least one active admin is required")
		return
	}
	if err := identity.Audit(r.Context(), tx, p.WorkspaceID, p.UserID, "role.change", r.PathValue("id"), "success", requestID(r)); err != nil {
		s.internal(w, r, err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.internal(w, r, err)
		return
	}
	respond(w, 200, map[string]string{"id": r.PathValue("id"), "role": input.Role})
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "admin")
	if !ok {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,COALESCE(actor_id::text,''),action,target,outcome,COALESCE(request_id,''),created_at FROM audit_events WHERE workspace_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, p.WorkspaceID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, actor, action, target, outcome, requestID string
		var created any
		if err := rows.Scan(&id, &actor, &action, &target, &outcome, &requestID, &created); err != nil {
			s.internal(w, r, err)
			return
		}
		items = append(items, map[string]any{"id": id, "actor_id": actor, "action": action, "target": target, "outcome": outcome, "request_id": requestID, "created_at": created})
	}
	if err := rows.Err(); err != nil {
		s.internal(w, r, err)
		return
	}
	respond(w, 200, map[string]any{"events": items})
}

var _ = sql.ErrNoRows
var _ = errors.Is
