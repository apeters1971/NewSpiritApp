package store

import (
	"fmt"
	"strings"
	"unicode"
)

func (s *Store) migrateUserBank() error {
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN iban TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN bic TEXT NOT NULL DEFAULT ''`)
	return nil
}

func normalizeIBAN(s string) (string, error) {
	s = compactBank(s)
	if s == "" {
		return "", nil
	}
	if len(s) < 15 || len(s) > 34 || !bankAlphaNum(s) || !unicode.IsLetter(rune(s[0])) || !unicode.IsLetter(rune(s[1])) {
		return "", fmt.Errorf("invalid iban")
	}
	return s, nil
}

func normalizeBIC(s string) (string, error) {
	s = compactBank(s)
	if s == "" {
		return "", nil
	}
	if (len(s) != 8 && len(s) != 11) || !bankAlphaNum(s) {
		return "", fmt.Errorf("invalid bic")
	}
	return s, nil
}

func compactBank(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) || r == '-' {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func bankAlphaNum(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func (s *Store) SetUserBank(id, iban, bic string) (User, error) {
	u, err := s.UserByID(id)
	if err != nil {
		return User{}, err
	}
	if !CanHaveBankDetails(u.Role) {
		iban, bic = "", ""
	}
	iban, err = normalizeIBAN(iban)
	if err != nil {
		return User{}, err
	}
	bic, err = normalizeBIC(bic)
	if err != nil {
		return User{}, err
	}
	if _, err := s.db.Exec(`UPDATE users SET iban=?, bic=? WHERE id=?`, iban, bic, id); err != nil {
		return User{}, err
	}
	return s.UserByID(id)
}

func (s *Store) clearUserBankIfNeeded(id, role string) error {
	if CanHaveBankDetails(role) {
		return nil
	}
	_, err := s.db.Exec(`UPDATE users SET iban='', bic='' WHERE id=?`, id)
	return err
}
