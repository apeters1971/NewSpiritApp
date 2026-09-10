package store

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	ChannelMin        = 1
	ChannelMax        = 96
	channelCommentMax = 200
)

type Channel struct {
	Number   int    `json:"number"`
	UserID   string `json:"userId,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Role     string `json:"role,omitempty"`
	Subrole  string `json:"subrole,omitempty"`
	Comment  string `json:"comment,omitempty"`
	V48      bool   `json:"v48"`
}

func CanHaveChannel(role string) bool {
	return role == RoleChoir || role == RoleChorleiter || role == RoleBand || role == RoleOrchestra
}

func (s *Store) migrateChannels() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS channels (
  number INTEGER PRIMARY KEY,
  user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  comment TEXT NOT NULL DEFAULT '',
  v48 INTEGER NOT NULL DEFAULT 0
)`); err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE channels ADD COLUMN v48 INTEGER NOT NULL DEFAULT 0`)
	for n := ChannelMin; n <= ChannelMax; n++ {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO channels(number, comment) VALUES(?, '')`, n); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListChannels() ([]Channel, error) {
	if err := s.migrateChannels(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
SELECT c.number, c.user_id, u.nickname, u.role, u.subrole, c.comment, c.v48
FROM channels c
LEFT JOIN users u ON u.id = c.user_id
ORDER BY c.number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Channel, 0, ChannelMax)
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func prepareChannelComment(comment string) (string, error) {
	comment = NormalizeName(comment)
	if utf8.RuneCountInString(comment) > channelCommentMax {
		return "", fmt.Errorf("channel comment is too long")
	}
	return comment, nil
}

func (s *Store) SetChannel(number int, userID, comment string, v48 bool) (Channel, error) {
	if number < ChannelMin || number > ChannelMax {
		return Channel{}, fmt.Errorf("unknown channel")
	}
	if err := s.migrateChannels(); err != nil {
		return Channel{}, err
	}
	comment, err := prepareChannelComment(comment)
	if err != nil {
		return Channel{}, err
	}
	userID = strings.TrimSpace(userID)
	var uid any
	if userID != "" {
		u, err := s.UserByID(userID)
		if err != nil {
			return Channel{}, err
		}
		if !CanHaveChannel(u.Role) {
			return Channel{}, fmt.Errorf("channel is only for choir, choir director, band, or orchestra")
		}
		uid = u.ID
	}
	if _, err := s.db.Exec(`UPDATE channels SET user_id=?, comment=?, v48=? WHERE number=?`, uid, comment, boolInt(v48), number); err != nil {
		return Channel{}, err
	}
	return s.channelRow(number)
}

func (s *Store) SetChannelComment(userID string, number int, comment string, v48 bool) (Channel, error) {
	if number < ChannelMin || number > ChannelMax {
		return Channel{}, fmt.Errorf("unknown channel")
	}
	comment, err := prepareChannelComment(comment)
	if err != nil {
		return Channel{}, err
	}
	ch, err := s.channelRow(number)
	if err != nil {
		return Channel{}, err
	}
	if ch.UserID == "" || ch.UserID != userID {
		return Channel{}, fmt.Errorf("%w: this channel is not yours", ErrForbidden)
	}
	if _, err := s.db.Exec(`UPDATE channels SET comment=?, v48=? WHERE number=? AND user_id=?`, comment, boolInt(v48), number, userID); err != nil {
		return Channel{}, err
	}
	return s.channelRow(number)
}

func (s *Store) ClearChannelsForUser(userID string) error {
	if userID == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE channels SET user_id=NULL WHERE user_id=?`, userID)
	return err
}

func (s *Store) channelRow(number int) (Channel, error) {
	row := s.db.QueryRow(`
SELECT c.number, c.user_id, u.nickname, u.role, u.subrole, c.comment, c.v48
FROM channels c
LEFT JOIN users u ON u.id = c.user_id
WHERE c.number=?`, number)
	ch, err := scanChannel(row)
	if err == sql.ErrNoRows {
		return Channel{}, ErrNotFound
	}
	return ch, err
}

func scanChannel(rs rowScanner) (Channel, error) {
	var ch Channel
	var userID, nickname, role, subrole sql.NullString
	var v48 int
	if err := rs.Scan(&ch.Number, &userID, &nickname, &role, &subrole, &ch.Comment, &v48); err != nil {
		return Channel{}, err
	}
	ch.V48 = v48 != 0
	if userID.Valid && strings.TrimSpace(userID.String) != "" {
		ch.UserID = userID.String
		ch.Nickname = nickname.String
		ch.Role = role.String
		ch.Subrole = subrole.String
	}
	return ch, nil
}

func (s *Store) attachChannels(users []User) error {
	if len(users) == 0 {
		return nil
	}
	rows, err := s.db.Query(`
SELECT c.number, c.user_id, u.nickname, u.role, u.subrole, c.comment, c.v48
FROM channels c
JOIN users u ON u.id = c.user_id
ORDER BY c.number`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil
		}
		return err
	}
	defer rows.Close()
	byUser := map[string][]Channel{}
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return err
		}
		byUser[ch.UserID] = append(byUser[ch.UserID], ch)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range users {
		users[i].Channels = byUser[users[i].ID]
		if users[i].Channels == nil {
			users[i].Channels = []Channel{}
		}
	}
	return nil
}

func (s *Store) attachChannelPtr(u *User) error {
	if u == nil || u.ID == "" {
		return nil
	}
	users := []User{*u}
	if err := s.attachChannels(users); err != nil {
		return err
	}
	u.Channels = users[0].Channels
	return nil
}
