package store

import (
	"database/sql"
	"fmt"
	"net/mail"
	"strings"
)

const (
	SettingAdminAlias = "admin_alias"
	SettingNewsTicker = "news_ticker"
	SettingMailFrom   = "mail_from"
	maxAdminAlias     = 40
	maxNewsTicker     = 400
	maxMailFrom       = 254
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

func NormalizeNewsTicker(text string) (string, error) {
	text = NormalizeName(text)
	if len([]rune(text)) > maxNewsTicker {
		return "", fmt.Errorf("news ticker is too long")
	}
	return text, nil
}

func (s *Store) NewsTicker() string {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, SettingNewsTicker).Scan(&value)
	if err != nil {
		return ""
	}
	text, err := NormalizeNewsTicker(value)
	if err != nil {
		return ""
	}
	return text
}

func (s *Store) SetNewsTicker(text string) (string, error) {
	text, err := NormalizeNewsTicker(text)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO settings(key, value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		SettingNewsTicker, text,
	)
	if err != nil {
		return "", err
	}
	return text, nil
}

func NormalizeMailFrom(from string) (string, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(from)
	if err != nil || addr.Address == "" {
		return "", fmt.Errorf("invalid mail from")
	}
	from = NormalizeEmail(addr.Address)
	if from == "" || !strings.Contains(from, "@") {
		return "", fmt.Errorf("invalid mail from")
	}
	if len(from) > maxMailFrom {
		return "", fmt.Errorf("invalid mail from")
	}
	return from, nil
}

func (s *Store) MailFrom() string {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, SettingMailFrom).Scan(&value)
	if err != nil {
		return ""
	}
	from, err := NormalizeMailFrom(value)
	if err != nil {
		return ""
	}
	return from
}

func (s *Store) SetMailFrom(from string) (string, error) {
	from, err := NormalizeMailFrom(from)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO settings(key, value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		SettingMailFrom, from,
	)
	if err != nil {
		return "", err
	}
	return from, nil
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
