package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Store) migrateCalendar() error {
	if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN calendar_token TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}
	_, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_calendar_token ON users(calendar_token) WHERE calendar_token != ''`)
	return err
}

func (s *Store) EnsureCalendarToken(userID string) (string, error) {
	var token string
	err := s.db.QueryRow(`SELECT calendar_token FROM users WHERE id=?`, userID).Scan(&token)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if token != "" {
		return token, nil
	}
	for range 5 {
		token = newID() + newID()
		res, err := s.db.Exec(`UPDATE users SET calendar_token=? WHERE id=? AND calendar_token=''`, token, userID)
		if err != nil {
			if isUnique(err) {
				continue
			}
			return "", err
		}
		n, _ := res.RowsAffected()
		if n == 1 {
			return token, nil
		}
		err = s.db.QueryRow(`SELECT calendar_token FROM users WHERE id=?`, userID).Scan(&token)
		if err != nil {
			return "", err
		}
		if token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("could not create calendar token")
}

func (s *Store) UserByCalendarToken(token string) (User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return User{}, ErrNotFound
	}
	var u User
	err := s.db.QueryRow(`SELECT id, nickname, role FROM users WHERE calendar_token=?`, token).Scan(&u.ID, &u.Nickname, &u.Role)
	if err == sql.ErrNoRows {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) ListAcceptedDatesForRole(role string) ([]Date, error) {
	dates, err := s.listDates()
	if err != nil {
		return nil, err
	}
	out := []Date{}
	for _, d := range dates {
		if d.Status != StatusAccepted || !slicesContains(d.Roles, role) {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

func RenderICS(name string, dates []Date) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//New Spirit//Dates//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString(foldICS("X-WR-CALNAME:"+icsText(name)) + "\r\n")
	now := icsTime(time.Now().UTC())
	for _, d := range dates {
		end := d.StartsAt.Add(2 * time.Hour)
		if d.EndsAt != nil {
			end = *d.EndsAt
		}
		desc := calendarDescription(d)
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString("UID:date-" + d.ID + "@newspirit\r\n")
		b.WriteString("DTSTAMP:" + now + "\r\n")
		b.WriteString("DTSTART:" + icsTime(d.StartsAt) + "\r\n")
		b.WriteString("DTEND:" + icsTime(end) + "\r\n")
		b.WriteString(foldICS("SUMMARY:"+icsText(d.Title)) + "\r\n")
		if d.Location != "" {
			b.WriteString(foldICS("LOCATION:"+icsText(d.Location)) + "\r\n")
		}
		if desc != "" {
			b.WriteString(foldICS("DESCRIPTION:"+icsText(desc)) + "\r\n")
		}
		b.WriteString("STATUS:CONFIRMED\r\n")
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func calendarDescription(d Date) string {
	parts := []string{}
	if label := CategoryLabels[d.Category]; label != "" {
		parts = append(parts, label)
	}
	if d.Notes != "" {
		parts = append(parts, d.Notes)
	}
	if d.Schedule != "" {
		parts = append(parts, d.Schedule)
	}
	return strings.Join(parts, "\n")
}

func icsTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func icsText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\r\n", `\n`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func foldICS(line string) string {
	b := []byte(line)
	if len(b) <= 75 {
		return line
	}
	var out strings.Builder
	limit := 75
	for len(b) > 0 {
		n := limit
		if n > len(b) {
			n = len(b)
		}
		for n > 0 && n < len(b) && b[n]&0xc0 == 0x80 {
			n--
		}
		if n == 0 {
			n = 1
		}
		if out.Len() > 0 {
			out.WriteString("\r\n ")
		}
		out.Write(b[:n])
		b = b[n:]
		limit = 74
	}
	return out.String()
}
