package store

import (
	"fmt"
	"strings"
)

const maxFeeCents = 100_000_000

func DateAllowsFee(roles []string) bool {
	return slicesContains(roles, RoleBand) || slicesContains(roles, RoleOrchestra)
}

func (s *Store) migrateDateFee() error {
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN fee_cents INTEGER NOT NULL DEFAULT 0`)
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS date_fee_overrides (
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  fee_cents INTEGER NOT NULL,
  PRIMARY KEY (date_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_date_fee_overrides_date ON date_fee_overrides(date_id);
`)
	return err
}

func NormalizeFeeCents(cents int) (int, error) {
	if cents < 0 {
		return 0, fmt.Errorf("fee cannot be negative")
	}
	if cents > maxFeeCents {
		return 0, fmt.Errorf("fee is too large")
	}
	return cents, nil
}

func (s *Store) SetDateFee(dateID string, feeCents int) error {
	d, err := s.dateRow(dateID)
	if err != nil {
		return err
	}
	cents, err := NormalizeFeeCents(feeCents)
	if err != nil {
		return err
	}
	if !DateAllowsFee(d.Roles) {
		cents = 0
		if _, err := s.db.Exec(`DELETE FROM date_fee_overrides WHERE date_id=?`, dateID); err != nil {
			return err
		}
	}
	_, err = s.db.Exec(`UPDATE dates SET fee_cents=? WHERE id=?`, cents, dateID)
	return err
}

func canOverrideDateFee(d Date, u User) bool {
	if !DateAllowsFee(d.Roles) {
		return false
	}
	if u.Role != RoleBand && u.Role != RoleOrchestra {
		return false
	}
	return neededOnRoster(d, u.Role, u.ID)
}

func applyViewerFee(d *Date, viewer *User) {
	if d == nil || viewer == nil {
		return
	}
	if d.FeeOverrides != nil {
		if cents, ok := d.FeeOverrides[viewer.ID]; ok {
			d.FeeCents = cents
		}
	}
	d.FeeOverrides = nil
}

func (s *Store) SetDateFeeOverrides(dateID string, overrides map[string]int) error {
	d, err := s.dateRow(dateID)
	if err != nil {
		return err
	}
	type feeRow struct {
		userID string
		cents  int
	}
	rows := make([]feeRow, 0, len(overrides))
	if DateAllowsFee(d.Roles) {
		for userID, cents := range overrides {
			userID = strings.TrimSpace(userID)
			if userID == "" {
				continue
			}
			cents, err = NormalizeFeeCents(cents)
			if err != nil {
				return err
			}
			u, err := s.UserByID(userID)
			if err != nil {
				if err == ErrNotFound {
					return fmt.Errorf("unknown person")
				}
				return err
			}
			if !canOverrideDateFee(d, u) {
				continue
			}
			rows = append(rows, feeRow{userID: userID, cents: cents})
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM date_fee_overrides WHERE date_id=?`, dateID); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := tx.Exec(
			`INSERT INTO date_fee_overrides(date_id, user_id, fee_cents) VALUES(?,?,?)`,
			dateID, row.userID, row.cents,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) feesForDates(ids []string) (map[string]map[string]int, error) {
	out := map[string]map[string]int{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT date_id, user_id, fee_cents FROM date_fee_overrides WHERE date_id IN (`+placeholders+`) ORDER BY user_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dateID, userID string
		var cents int
		if err := rows.Scan(&dateID, &userID, &cents); err != nil {
			return nil, err
		}
		if out[dateID] == nil {
			out[dateID] = map[string]int{}
		}
		out[dateID][userID] = cents
	}
	return out, rows.Err()
}
