package store

import (
	"fmt"
	"slices"
	"strings"
)

func NeededPickRoles() []string {
	return []string{RoleBand, RoleOrchestra}
}

func IsNeededPickRole(role string) bool {
	return role == RoleBand || role == RoleOrchestra
}

func UserSeesDate(user User, d Date) bool {
	if user.ID != "" && d.CreatedBy != "" && d.CreatedBy == user.ID {
		return true
	}
	if IsLocationOwner(user.Role) {
		return OwnsVenue(user, d)
	}
	if !RoleSeesDate(user.Role, d.Roles) {
		return false
	}
	return neededOnRoster(d, user.Role, user.ID)
}

func neededOnRoster(d Date, role, userID string) bool {
	if !IsNeededPickRole(role) {
		return true
	}
	if !slices.Contains(d.NeededRoles, role) {
		return true
	}
	return slices.Contains(d.NeededIDs, userID)
}

func (s *Store) migrateDateNeeded() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS date_needed (
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (date_id, user_id)
);
CREATE TABLE IF NOT EXISTS date_needed_roles (
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  PRIMARY KEY (date_id, role)
);
CREATE INDEX IF NOT EXISTS idx_date_needed_date ON date_needed(date_id);
`)
	return err
}

func (s *Store) SetDateNeeded(dateID string, neededRoles, neededIDs []string) error {
	d, err := s.dateRow(dateID)
	if err != nil {
		return err
	}
	roles, ids, err := s.cleanDateNeeded(d.Roles, neededRoles, neededIDs)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM date_needed WHERE date_id=?`, dateID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM date_needed_roles WHERE date_id=?`, dateID); err != nil {
		return err
	}
	for _, role := range roles {
		if _, err := tx.Exec(`INSERT INTO date_needed_roles(date_id, role) VALUES(?,?)`, dateID, role); err != nil {
			return err
		}
	}
	for _, id := range ids {
		if _, err := tx.Exec(`INSERT INTO date_needed(date_id, user_id) VALUES(?,?)`, dateID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) cleanDateNeeded(dateRoles, neededRoles, neededIDs []string) ([]string, []string, error) {
	allowed := map[string]bool{}
	for _, role := range NeededPickRoles() {
		if slices.Contains(dateRoles, role) {
			allowed[role] = true
		}
	}
	seenRole := map[string]bool{}
	roles := make([]string, 0, len(neededRoles))
	for _, role := range neededRoles {
		role, err := NormalizeRole(role)
		if err != nil || !allowed[role] {
			continue
		}
		if seenRole[role] {
			continue
		}
		seenRole[role] = true
		roles = append(roles, role)
	}
	slices.Sort(roles)
	seenID := map[string]bool{}
	ids := make([]string, 0, len(neededIDs))
	for _, id := range neededIDs {
		id = strings.TrimSpace(id)
		if id == "" || seenID[id] {
			continue
		}
		u, err := s.UserByID(id)
		if err != nil {
			if err == ErrNotFound {
				return nil, nil, fmt.Errorf("unknown person")
			}
			return nil, nil, err
		}
		if !seenRole[u.Role] {
			continue
		}
		seenID[id] = true
		ids = append(ids, id)
	}
	return roles, ids, nil
}

func (s *Store) neededForDates(ids []string) (map[string][]string, map[string][]string, error) {
	people := map[string][]string{}
	roles := map[string][]string{}
	if len(ids) == 0 {
		return people, roles, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT date_id, user_id FROM date_needed WHERE date_id IN (`+placeholders+`) ORDER BY user_id`, args...)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var dateID, userID string
		if err := rows.Scan(&dateID, &userID); err != nil {
			rows.Close()
			return nil, nil, err
		}
		people[dateID] = append(people[dateID], userID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT date_id, role FROM date_needed_roles WHERE date_id IN (`+placeholders+`) ORDER BY role`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dateID, role string
		if err := rows.Scan(&dateID, &role); err != nil {
			return nil, nil, err
		}
		roles[dateID] = append(roles[dateID], role)
	}
	return people, roles, rows.Err()
}
