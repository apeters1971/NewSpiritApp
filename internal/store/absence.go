package store

import (
	"database/sql"
	"fmt"
	"strings"
)

const (
	AttendanceAbsent  = "absent"
	AttendanceExcused = "excused"
)

func ValidAttendance(kind string) bool {
	return kind == "" || kind == AttendanceAbsent || kind == AttendanceExcused
}

func (s *Store) migrateAbsences() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS date_absences (
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL DEFAULT 'absent',
  PRIMARY KEY (date_id, user_id)
)`); err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE date_absences ADD COLUMN kind TEXT NOT NULL DEFAULT 'absent'`)
	return nil
}

func (s *Store) SetDateAttendance(dateID, userID, kind string) error {
	if !ValidAttendance(kind) {
		return fmt.Errorf("invalid attendance")
	}
	if _, err := s.dateRow(dateID); err != nil {
		return err
	}
	if _, err := s.UserByID(userID); err != nil {
		return err
	}
	if kind == "" {
		_, err := s.db.Exec(`DELETE FROM date_absences WHERE date_id=? AND user_id=?`, dateID, userID)
		return err
	}
	var choice string
	err := s.db.QueryRow(`SELECT choice FROM votes WHERE date_id=? AND user_id=?`, dateID, userID).Scan(&choice)
	if err != nil || choice != VoteYes {
		return fmt.Errorf("only a yes vote can be marked absent")
	}
	_, err = s.db.Exec(`
INSERT INTO date_absences(date_id, user_id, kind) VALUES(?,?,?)
ON CONFLICT(date_id, user_id) DO UPDATE SET kind=excluded.kind`, dateID, userID, kind)
	return err
}

func (s *Store) attendanceForDates(ids []string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT date_id, user_id, kind FROM date_absences WHERE date_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dateID, userID, kind string
		if err := rows.Scan(&dateID, &userID, &kind); err != nil {
			return nil, err
		}
		if !ValidAttendance(kind) || kind == "" {
			continue
		}
		if out[dateID] == nil {
			out[dateID] = map[string]string{}
		}
		out[dateID][userID] = kind
	}
	return out, rows.Err()
}

func scanAttendance(kind sql.NullString) string {
	if kind.Valid && ValidAttendance(kind.String) {
		return kind.String
	}
	return ""
}
