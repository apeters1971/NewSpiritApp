package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	VoteLinkLife      = 7 * 24 * time.Hour
	VoteLinkUseWindow = 5 * time.Minute
)

var ErrVoteLinkGone = fmt.Errorf("%w: vote link expired", ErrForbidden)

type VoteReminderLink struct {
	UserID   string
	Nickname string
	Email    string
	Token    string
}

type VoteReminderSession struct {
	User      User
	Date      Date
	Remaining time.Duration
}

func voteTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) migrateVoteReminders() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS vote_reminders (
  id TEXT PRIMARY KEY,
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  first_used_at TEXT NOT NULL DEFAULT '',
  used_expires_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_vote_reminders_date ON vote_reminders(date_id);
`)
	return err
}

func (s *Store) CanSendVoteReminder(user User, dateID string) (Date, error) {
	d, err := s.dateRow(dateID)
	if err != nil {
		return Date{}, err
	}
	if !IsChoirDirector(user.Role) {
		return Date{}, fmt.Errorf("%w: only the choir director can send reminders", ErrForbidden)
	}
	if !UserSeesDate(user, d) {
		return Date{}, fmt.Errorf("%w: this date is not for your role", ErrForbidden)
	}
	if !DateIsCurrent(d) {
		return Date{}, fmt.Errorf("voting is closed")
	}
	return d, nil
}

func (s *Store) ReplaceVoteReminderLinks(dateID string) ([]VoteReminderLink, error) {
	d, err := s.dateRow(dateID)
	if err != nil {
		return nil, err
	}
	if !DateIsCurrent(d) {
		return nil, fmt.Errorf("voting is closed")
	}
	people, err := s.roster(d)
	if err != nil {
		return nil, err
	}
	now := now()
	exp := now.Add(VoteLinkLife)
	out := make([]VoteReminderLink, 0, len(people))
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM vote_reminders WHERE date_id=?`, dateID); err != nil {
		return nil, err
	}
	for _, e := range people {
		email := NormalizeEmail(e.Email)
		if email == "" || !strings.Contains(email, "@") {
			continue
		}
		if !OwnsVenue(User{ID: e.UserID, Role: e.Role}, d) {
			if !RoleCanVote(e.Role) || !slicesContains(d.Roles, e.Role) || !neededOnRoster(d, e.Role, e.UserID) {
				continue
			}
		}
		token := newID() + newID()
		if _, err := tx.Exec(
			`INSERT INTO vote_reminders(id, date_id, user_id, token_hash, created_at, expires_at) VALUES(?,?,?,?,?,?)`,
			newID(), dateID, e.UserID, voteTokenHash(token), fmtTime(now), fmtTime(exp),
		); err != nil {
			return nil, err
		}
		out = append(out, VoteReminderLink{
			UserID:   e.UserID,
			Nickname: e.Nickname,
			Email:    email,
			Token:    token,
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no one to mail")
	}
	return out, nil
}

func (s *Store) OpenVoteReminder(token string) (VoteReminderSession, error) {
	sess, rec, err := s.lookupVoteReminder(token)
	if err != nil {
		return VoteReminderSession{}, err
	}
	if rec.firstUsed == "" {
		used := now()
		until := used.Add(VoteLinkUseWindow)
		if _, err := s.db.Exec(
			`UPDATE vote_reminders SET first_used_at=?, used_expires_at=? WHERE id=? AND first_used_at=''`,
			fmtTime(used), fmtTime(until), rec.id,
		); err != nil {
			return VoteReminderSession{}, err
		}
		sess.Remaining = VoteLinkUseWindow
		return sess, nil
	}
	return sess, nil
}

func (s *Store) VoteReminderSession(token string) (VoteReminderSession, error) {
	sess, _, err := s.lookupVoteReminder(token)
	return sess, err
}

type voteReminderRow struct {
	id        string
	firstUsed string
}

func (s *Store) lookupVoteReminder(token string) (VoteReminderSession, voteReminderRow, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return VoteReminderSession{}, voteReminderRow{}, ErrVoteLinkGone
	}
	var rec voteReminderRow
	var dateID, userID, expires, usedExp string
	err := s.db.QueryRow(`
SELECT id, date_id, user_id, expires_at, first_used_at, used_expires_at
FROM vote_reminders WHERE token_hash=?`, voteTokenHash(token)).Scan(
		&rec.id, &dateID, &userID, &expires, &rec.firstUsed, &usedExp,
	)
	if err != nil {
		return VoteReminderSession{}, voteReminderRow{}, ErrVoteLinkGone
	}
	now := now()
	if !parseTime(expires).After(now) {
		return VoteReminderSession{}, voteReminderRow{}, ErrVoteLinkGone
	}
	remain := VoteLinkUseWindow
	if rec.firstUsed != "" {
		until := parseTime(usedExp)
		if !until.After(now) {
			return VoteReminderSession{}, voteReminderRow{}, ErrVoteLinkGone
		}
		remain = until.Sub(now)
	}
	u, err := s.UserByID(userID)
	if err != nil {
		return VoteReminderSession{}, voteReminderRow{}, ErrVoteLinkGone
	}
	d, err := s.dateRow(dateID)
	if err != nil {
		return VoteReminderSession{}, voteReminderRow{}, ErrVoteLinkGone
	}
	return VoteReminderSession{User: u, Date: d, Remaining: remain}, rec, nil
}

func (s *Store) forceVoteLinkTimes(token string, expiresAt, firstUsed, usedExp time.Time) error {
	_, err := s.db.Exec(
		`UPDATE vote_reminders SET expires_at=?, first_used_at=?, used_expires_at=? WHERE token_hash=?`,
		fmtTime(expiresAt), fmtTime(firstUsed), fmtTime(usedExp), voteTokenHash(token),
	)
	return err
}

func FormatChoirWhen(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(choirZone()).Format("Mon, 2 Jan 2006, 15:04")
}
