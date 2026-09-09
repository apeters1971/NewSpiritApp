package store

import (
	"database/sql"
	"fmt"
)

const (
	SettingAdminAlias = "admin_alias"
	maxAdminAlias     = 40
)

func NormalizeAdminAlias(alias string) (string, error) {
	alias = NormalizeName(alias)
	if alias == "" {
		return ChatAdminName, nil
	}
	if len([]rune(alias)) > maxAdminAlias {
		return "", fmt.Errorf("admin alias is too long")
	}
	return alias, nil
}

func (s *Store) AdminAlias() string {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, SettingAdminAlias).Scan(&value)
	if err == sql.ErrNoRows {
		return ChatAdminName
	}
	if err != nil {
		return ChatAdminName
	}
	alias, err := NormalizeAdminAlias(value)
	if err != nil {
		return ChatAdminName
	}
	return alias
}

func (s *Store) SetAdminAlias(alias string) (string, error) {
	alias, err := NormalizeAdminAlias(alias)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO settings(key, value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		SettingAdminAlias, alias,
	)
	if err != nil {
		return "", err
	}
	return alias, nil
}

func (s *Store) withAdminAlias(msgs []ChatMessage) []ChatMessage {
	alias := s.AdminAlias()
	for i := range msgs {
		if msgs[i].IsAdmin {
			msgs[i].Nickname = alias
		}
	}
	return msgs
}
