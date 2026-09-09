package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID        string    `json:"id"`
	Nickname  string    `json:"nickname"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Subrole   string    `json:"subrole"`
	CreatedAt time.Time `json:"createdAt"`
}

type Date struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Category  string     `json:"category"`
	StartsAt  time.Time  `json:"startsAt"`
	EndsAt    *time.Time `json:"endsAt,omitempty"`
	Location  string     `json:"location,omitempty"`
	Notes     string     `json:"notes,omitempty"`
	Status    string     `json:"status"`
	Roles     []string   `json:"roles"`
	Bring     Bring      `json:"bring"`
	CreatedAt time.Time  `json:"createdAt"`
}

type Bring struct {
	Mic   bool   `json:"mic"`
	Cable bool   `json:"cable"`
	Stand bool   `json:"stand"`
	Dress string `json:"dress,omitempty"`
}

type RosterEntry struct {
	UserID        string  `json:"userId"`
	Nickname      string  `json:"nickname"`
	Email         string  `json:"email"`
	Role          string  `json:"role"`
	Subrole       string  `json:"subrole"`
	Choice        string  `json:"choice"`
	InitialChoice *string `json:"initialChoice,omitempty"`
}

type SubroleCount struct {
	Role    string `json:"role"`
	Subrole string `json:"subrole"`
	Yes     int    `json:"yes"`
	Maybe   int    `json:"maybe"`
	No      int    `json:"no"`
	Unknown int    `json:"unknown"`
	Total   int    `json:"total"`
}

type Comment struct {
	ID        string    `json:"id"`
	DateID    string    `json:"dateId"`
	UserID    string    `json:"userId"`
	Nickname  string    `json:"nickname"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type DateView struct {
	Date
	MyChoice      string         `json:"myChoice"`
	MyInitial     *string        `json:"myInitial,omitempty"`
	Roster        []RosterEntry  `json:"roster"`
	SubroleCounts []SubroleCount `json:"subroleCounts"`
	Comments      []Comment      `json:"comments"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(8000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  nickname TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL,
  subrole TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  revoked_at TEXT
);
CREATE TABLE IF NOT EXISTS controller_sessions (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  revoked_at TEXT
);
CREATE TABLE IF NOT EXISTS dates (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  category TEXT NOT NULL DEFAULT 'event',
  starts_at TEXT NOT NULL,
  ends_at TEXT,
  location TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  bring_mic INTEGER NOT NULL DEFAULT 0,
  bring_cable INTEGER NOT NULL DEFAULT 0,
  bring_stand INTEGER NOT NULL DEFAULT 0,
  bring_dress TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS date_roles (
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  PRIMARY KEY (date_id, role)
);
CREATE TABLE IF NOT EXISTS votes (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  choice TEXT NOT NULL,
  initial_choice TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, date_id)
);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_dates_starts ON dates(starts_at);
CREATE INDEX IF NOT EXISTS idx_votes_date ON votes(date_id);
CREATE TABLE IF NOT EXISTS comments (
  id TEXT PRIMARY KEY,
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  text TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_date ON comments(date_id, created_at);
`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN category TEXT NOT NULL DEFAULT 'event'`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN bring_mic INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN bring_cable INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN bring_stand INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN bring_dress TEXT NOT NULL DEFAULT ''`)
	return nil
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func now() time.Time { return time.Now().UTC() }

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}

func parseTimePtr(s sql.NullString) *time.Time {
	if !s.Valid || strings.TrimSpace(s.String) == "" {
		return nil
	}
	t := parseTime(s.String)
	return &t
}

var spaceRe = regexp.MustCompile(`\s+`)

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func NormalizeName(s string) string {
	s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
	return s
}

func hashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Store) CreateUser(nickname, email, password, role, subrole string) (User, error) {
	nickname = NormalizeName(nickname)
	email = NormalizeEmail(email)
	role, err := NormalizeRole(role)
	if err != nil {
		return User{}, err
	}
	subrole = strings.TrimSpace(subrole)
	if nickname == "" {
		return User{}, fmt.Errorf("nickname is required")
	}
	if email == "" || !strings.Contains(email, "@") {
		return User{}, fmt.Errorf("invalid email")
	}
	if len(password) < 6 {
		return User{}, fmt.Errorf("password must be at least 6 characters")
	}
	if !ValidSubrole(role, subrole) {
		return User{}, fmt.Errorf("invalid subrole for %s", RoleLabels[role])
	}
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	u := User{ID: newID(), Nickname: nickname, Email: email, Role: role, Subrole: subrole, CreatedAt: now()}
	_, err = s.db.Exec(
		`INSERT INTO users(id, nickname, email, password_hash, role, subrole, created_at) VALUES(?,?,?,?,?,?,?)`,
		u.ID, u.Nickname, u.Email, hash, u.Role, u.Subrole, fmtTime(u.CreatedAt),
	)
	if err != nil {
		if isUnique(err) {
			return User{}, fmt.Errorf("%w: nickname or email already exists", ErrConflict)
		}
		return User{}, err
	}
	return u, nil
}

func (s *Store) UpdateUser(id, nickname, email, password, role, subrole string) (User, error) {
	cur, err := s.UserByID(id)
	if err != nil {
		return User{}, err
	}
	if nickname = NormalizeName(nickname); nickname == "" {
		nickname = cur.Nickname
	}
	if email = NormalizeEmail(email); email == "" {
		email = cur.Email
	} else if !strings.Contains(email, "@") {
		return User{}, fmt.Errorf("invalid email")
	}
	if role == "" {
		role = cur.Role
	} else if role, err = NormalizeRole(role); err != nil {
		return User{}, err
	}
	if subrole = strings.TrimSpace(subrole); subrole == "" {
		subrole = cur.Subrole
	}
	if !ValidSubrole(role, subrole) {
		return User{}, fmt.Errorf("invalid subrole for %s", RoleLabels[role])
	}

	if password != "" {
		if len(password) < 6 {
			return User{}, fmt.Errorf("password must be at least 6 characters")
		}
		hash, err := hashPassword(password)
		if err != nil {
			return User{}, err
		}
		_, err = s.db.Exec(
			`UPDATE users SET nickname=?, email=?, password_hash=?, role=?, subrole=? WHERE id=?`,
			nickname, email, hash, role, subrole, id,
		)
		if err != nil {
			if isUnique(err) {
				return User{}, fmt.Errorf("%w: nickname or email already exists", ErrConflict)
			}
			return User{}, err
		}
	} else {
		_, err = s.db.Exec(
			`UPDATE users SET nickname=?, email=?, role=?, subrole=? WHERE id=?`,
			nickname, email, role, subrole, id,
		)
		if err != nil {
			if isUnique(err) {
				return User{}, fmt.Errorf("%w: nickname or email already exists", ErrConflict)
			}
			return User{}, err
		}
	}
	return s.UserByID(id)
}

func (s *Store) DeleteUser(id string) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, nickname, email, role, subrole, created_at FROM users ORDER BY nickname COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UserByID(id string) (User, error) {
	return scanUserRow(s.db.QueryRow(`SELECT id, nickname, email, role, subrole, created_at FROM users WHERE id=?`, id))
}

func (s *Store) Login(email, password string) (User, string, error) {
	email = NormalizeEmail(email)
	if email == "" || password == "" {
		return User{}, "", ErrUnauthorized
	}
	var u User
	var hash, created string
	err := s.db.QueryRow(
		`SELECT id, nickname, email, password_hash, role, subrole, created_at FROM users WHERE email=?`,
		email,
	).Scan(&u.ID, &u.Nickname, &u.Email, &hash, &u.Role, &u.Subrole, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrUnauthorized
	}
	if err != nil {
		return User{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, "", ErrUnauthorized
	}
	u.CreatedAt = parseTime(created)
	sid := newID()
	if _, err := s.db.Exec(`INSERT INTO sessions(id, user_id, created_at) VALUES(?,?,?)`, sid, u.ID, fmtTime(now())); err != nil {
		return User{}, "", err
	}
	return u, sid, nil
}

func (s *Store) UserBySession(sessionID string) (User, error) {
	if sessionID == "" {
		return User{}, ErrUnauthorized
	}
	u, err := scanUserRow(s.db.QueryRow(`
SELECT u.id, u.nickname, u.email, u.role, u.subrole, u.created_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id=? AND s.revoked_at IS NULL`, sessionID))
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrUnauthorized
	}
	return u, err
}

func (s *Store) RevokeSession(sessionID string) {
	if sessionID == "" {
		return
	}
	_, _ = s.db.Exec(`UPDATE sessions SET revoked_at=? WHERE id=? AND revoked_at IS NULL`, fmtTime(now()), sessionID)
}

func (s *Store) CreateControllerSession() (string, error) {
	sid := newID()
	_, err := s.db.Exec(`INSERT INTO controller_sessions(id, created_at) VALUES(?,?)`, sid, fmtTime(now()))
	return sid, err
}

func (s *Store) ValidControllerSession(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM controller_sessions WHERE id=? AND revoked_at IS NULL`, sessionID).Scan(&n)
	return err == nil && n == 1
}

func (s *Store) RevokeControllerSession(sessionID string) {
	if sessionID == "" {
		return
	}
	_, _ = s.db.Exec(`UPDATE controller_sessions SET revoked_at=? WHERE id=? AND revoked_at IS NULL`, fmtTime(now()), sessionID)
}

func (s *Store) CreateDate(title, category string, startsAt time.Time, endsAt *time.Time, location, notes string, roles []string, bring Bring) (Date, error) {
	title = NormalizeName(title)
	if title == "" {
		return Date{}, fmt.Errorf("title is required")
	}
	if startsAt.IsZero() {
		return Date{}, fmt.Errorf("start time is required")
	}
	category, err := NormalizeCategory(category)
	if err != nil {
		return Date{}, err
	}
	roles, err = NormalizeRoles(roles)
	if err != nil {
		return Date{}, err
	}
	if endsAt != nil && endsAt.Before(startsAt) {
		return Date{}, fmt.Errorf("end time is before start time")
	}
	bring, err = normalizeBring(bring)
	if err != nil {
		return Date{}, err
	}
	d := Date{
		ID:        newID(),
		Title:     title,
		Category:  category,
		StartsAt:  startsAt.UTC(),
		EndsAt:    utcPtr(endsAt),
		Location:  NormalizeName(location),
		Notes:     strings.TrimSpace(notes),
		Status:    StatusVoting,
		Roles:     roles,
		Bring:     bring,
		CreatedAt: now(),
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Date{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var ends any
	if d.EndsAt != nil {
		ends = fmtTime(*d.EndsAt)
	}
	if _, err := tx.Exec(
		`INSERT INTO dates(id, title, category, starts_at, ends_at, location, notes, status, bring_mic, bring_cable, bring_stand, bring_dress, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.Title, d.Category, fmtTime(d.StartsAt), ends, d.Location, d.Notes, d.Status, boolInt(d.Bring.Mic), boolInt(d.Bring.Cable), boolInt(d.Bring.Stand), d.Bring.Dress, fmtTime(d.CreatedAt),
	); err != nil {
		return Date{}, err
	}
	if err := insertDateRoles(tx, d.ID, d.Roles); err != nil {
		return Date{}, err
	}
	if err := tx.Commit(); err != nil {
		return Date{}, err
	}
	return d, nil
}

func (s *Store) UpdateDate(id, title, category string, startsAt time.Time, endsAt *time.Time, location, notes string, roles []string, bring Bring) (Date, error) {
	if _, err := s.dateRow(id); err != nil {
		return Date{}, err
	}
	title = NormalizeName(title)
	if title == "" {
		return Date{}, fmt.Errorf("title is required")
	}
	if startsAt.IsZero() {
		return Date{}, fmt.Errorf("start time is required")
	}
	category, err := NormalizeCategory(category)
	if err != nil {
		return Date{}, err
	}
	roles, err = NormalizeRoles(roles)
	if err != nil {
		return Date{}, err
	}
	if endsAt != nil && endsAt.Before(startsAt) {
		return Date{}, fmt.Errorf("end time is before start time")
	}
	bring, err = normalizeBring(bring)
	if err != nil {
		return Date{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Date{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var ends any
	if endsAt != nil {
		ends = fmtTime(endsAt.UTC())
	}
	if _, err := tx.Exec(
		`UPDATE dates SET title=?, category=?, starts_at=?, ends_at=?, location=?, notes=?, bring_mic=?, bring_cable=?, bring_stand=?, bring_dress=? WHERE id=?`,
		title, category, fmtTime(startsAt.UTC()), ends, NormalizeName(location), strings.TrimSpace(notes), boolInt(bring.Mic), boolInt(bring.Cable), boolInt(bring.Stand), bring.Dress, id,
	); err != nil {
		return Date{}, err
	}
	if _, err := tx.Exec(`DELETE FROM date_roles WHERE date_id=?`, id); err != nil {
		return Date{}, err
	}
	if err := insertDateRoles(tx, id, roles); err != nil {
		return Date{}, err
	}
	if err := tx.Commit(); err != nil {
		return Date{}, err
	}
	return s.dateRow(id)
}

func (s *Store) DeleteDate(id string) error {
	res, err := s.db.Exec(`DELETE FROM dates WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetDateStatus(id, status string) (Date, error) {
	if !ValidStatus(status) || status == StatusVoting {
		return Date{}, fmt.Errorf("status must be accepted or cancelled")
	}
	d, err := s.dateRow(id)
	if err != nil {
		return Date{}, err
	}
	if d.Status != StatusVoting {
		return Date{}, fmt.Errorf("date is already %s", d.Status)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Date{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE dates SET status=? WHERE id=?`, status, id); err != nil {
		return Date{}, err
	}
	if err := tx.Commit(); err != nil {
		return Date{}, err
	}
	return s.dateRow(id)
}

func (s *Store) SetVote(userID, dateID, choice string) error {
	if !ValidChoice(choice) {
		return fmt.Errorf("invalid vote")
	}
	d, err := s.dateRow(dateID)
	if err != nil {
		return err
	}
	if d.Status == StatusCancelled {
		return fmt.Errorf("%w: voting is locked on cancelled dates", ErrForbidden)
	}
	u, err := s.UserByID(userID)
	if err != nil {
		return err
	}
	if !slicesContains(d.Roles, u.Role) {
		return fmt.Errorf("%w: this date is not for your role", ErrForbidden)
	}
	_, err = s.db.Exec(`
INSERT INTO votes(user_id, date_id, choice, initial_choice, updated_at) VALUES(?,?,?,?,?)
ON CONFLICT(user_id, date_id) DO UPDATE SET choice=excluded.choice, updated_at=excluded.updated_at`,
		userID, dateID, choice, choice, fmtTime(now()),
	)
	return err
}

func (s *Store) AddComment(userID, dateID, text string) (Comment, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Comment{}, fmt.Errorf("comment is required")
	}
	if len([]rune(text)) > 2000 {
		return Comment{}, fmt.Errorf("comment is too long")
	}
	d, err := s.dateRow(dateID)
	if err != nil {
		return Comment{}, err
	}
	u, err := s.UserByID(userID)
	if err != nil {
		return Comment{}, err
	}
	if !slicesContains(d.Roles, u.Role) {
		return Comment{}, fmt.Errorf("%w: this date is not for your role", ErrForbidden)
	}
	c := Comment{
		ID:        newID(),
		DateID:    dateID,
		UserID:    userID,
		Nickname:  u.Nickname,
		Text:      text,
		CreatedAt: now(),
	}
	_, err = s.db.Exec(
		`INSERT INTO comments(id, date_id, user_id, text, created_at) VALUES(?,?,?,?,?)`,
		c.ID, c.DateID, c.UserID, c.Text, fmtTime(c.CreatedAt),
	)
	return c, err
}

func (s *Store) commentsForDates(ids []string) (map[string][]Comment, error) {
	out := map[string][]Comment{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`
SELECT c.id, c.date_id, c.user_id, u.nickname, c.text, c.created_at
FROM comments c
JOIN users u ON u.id = c.user_id
WHERE c.date_id IN (`+placeholders+`)
ORDER BY c.created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Comment
		var created string
		if err := rows.Scan(&c.ID, &c.DateID, &c.UserID, &c.Nickname, &c.Text, &created); err != nil {
			return nil, err
		}
		c.CreatedAt = parseTime(created)
		out[c.DateID] = append(out[c.DateID], c)
	}
	return out, rows.Err()
}

func (s *Store) DateView(id string, viewer *User) (DateView, error) {
	d, err := s.dateRow(id)
	if err != nil {
		return DateView{}, err
	}
	commentsByDate, err := s.commentsForDates([]string{id})
	if err != nil {
		return DateView{}, err
	}
	return s.attachView(d, viewer, commentsByDate[id])
}

func (s *Store) ListDateViews(viewer *User) ([]DateView, error) {
	dates, err := s.listDates()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(dates))
	for _, d := range dates {
		if viewer != nil && !slicesContains(d.Roles, viewer.Role) {
			continue
		}
		ids = append(ids, d.ID)
	}
	commentsByDate, err := s.commentsForDates(ids)
	if err != nil {
		return nil, err
	}
	out := make([]DateView, 0, len(ids))
	for _, d := range dates {
		if viewer != nil && !slicesContains(d.Roles, viewer.Role) {
			continue
		}
		v, err := s.attachView(d, viewer, commentsByDate[d.ID])
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) attachView(d Date, viewer *User, comments []Comment) (DateView, error) {
	roster, err := s.roster(d)
	if err != nil {
		return DateView{}, err
	}
	if comments == nil {
		comments = []Comment{}
	}
	view := DateView{
		Date:          d,
		MyChoice:      VoteUnknown,
		Roster:        roster,
		SubroleCounts: countsFor(d.Roles, roster),
		Comments:      comments,
	}
	if viewer != nil {
		for _, entry := range roster {
			if entry.UserID == viewer.ID {
				view.MyChoice = entry.Choice
				view.MyInitial = entry.InitialChoice
				break
			}
		}
	}
	return view, nil
}

func (s *Store) listDates() ([]Date, error) {
	rows, err := s.db.Query(`SELECT id, title, category, starts_at, ends_at, location, notes, status, bring_mic, bring_cable, bring_stand, bring_dress, created_at FROM dates ORDER BY starts_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	dates := []Date{}
	ids := []string{}
	for rows.Next() {
		d, err := scanDate(rows)
		if err != nil {
			return nil, err
		}
		dates = append(dates, d)
		ids = append(ids, d.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	roleMap, err := s.rolesForDates(ids)
	if err != nil {
		return nil, err
	}
	for i := range dates {
		dates[i].Roles = roleMap[dates[i].ID]
		if dates[i].Roles == nil {
			dates[i].Roles = []string{}
		}
	}
	return dates, nil
}

func (s *Store) dateRow(id string) (Date, error) {
	d, err := scanDateRow(s.db.QueryRow(
		`SELECT id, title, category, starts_at, ends_at, location, notes, status, bring_mic, bring_cable, bring_stand, bring_dress, created_at FROM dates WHERE id=?`, id,
	))
	if err != nil {
		return Date{}, err
	}
	roles, err := s.rolesForDates([]string{id})
	if err != nil {
		return Date{}, err
	}
	d.Roles = roles[id]
	if d.Roles == nil {
		d.Roles = []string{}
	}
	return d, nil
}

func (s *Store) rolesForDates(ids []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT date_id, role FROM date_roles WHERE date_id IN (`+placeholders+`) ORDER BY role`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dateID, role string
		if err := rows.Scan(&dateID, &role); err != nil {
			return nil, err
		}
		out[dateID] = append(out[dateID], role)
	}
	return out, rows.Err()
}

func (s *Store) roster(d Date) ([]RosterEntry, error) {
	if len(d.Roles) == 0 {
		return []RosterEntry{}, nil
	}
	placeholders := strings.Repeat("?,", len(d.Roles))
	placeholders = placeholders[:len(placeholders)-1]
	args := []any{d.ID}
	for _, role := range d.Roles {
		args = append(args, role)
	}
	rows, err := s.db.Query(`
SELECT u.id, u.nickname, u.email, u.role, u.subrole, v.choice, v.initial_choice
FROM users u
LEFT JOIN votes v ON v.user_id = u.id AND v.date_id = ?
WHERE u.role IN (`+placeholders+`)
ORDER BY u.role, u.subrole, u.nickname COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RosterEntry{}
	for rows.Next() {
		var e RosterEntry
		var choice, initial sql.NullString
		if err := rows.Scan(&e.UserID, &e.Nickname, &e.Email, &e.Role, &e.Subrole, &choice, &initial); err != nil {
			return nil, err
		}
		e.Choice = VoteUnknown
		if choice.Valid && choice.String != "" {
			e.Choice = choice.String
		}
		if initial.Valid && initial.String != "" {
			s := initial.String
			e.InitialChoice = &s
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func countsFor(roles []string, roster []RosterEntry) []SubroleCount {
	index := map[string]int{}
	out := []SubroleCount{}
	for _, role := range roles {
		for _, sub := range Subroles[role] {
			index[role+"|"+sub] = len(out)
			out = append(out, SubroleCount{Role: role, Subrole: sub})
		}
	}
	for _, e := range roster {
		i, ok := index[e.Role+"|"+e.Subrole]
		if !ok {
			continue
		}
		out[i].Total++
		switch e.Choice {
		case VoteYes:
			out[i].Yes++
		case VoteMaybe:
			out[i].Maybe++
		case VoteNo:
			out[i].No++
		default:
			out[i].Unknown++
		}
	}
	kept := out[:0]
	for _, c := range out {
		if c.Total > 0 {
			kept = append(kept, c)
		}
	}
	return kept
}

func insertDateRoles(tx *sql.Tx, dateID string, roles []string) error {
	for _, role := range roles {
		if _, err := tx.Exec(`INSERT INTO date_roles(date_id, role) VALUES(?,?)`, dateID, role); err != nil {
			return err
		}
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(rs rowScanner) (User, error) {
	var u User
	var created string
	if err := rs.Scan(&u.ID, &u.Nickname, &u.Email, &u.Role, &u.Subrole, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	u.CreatedAt = parseTime(created)
	return u, nil
}

func scanUserRow(row *sql.Row) (User, error) { return scanUser(row) }

func scanDate(rs rowScanner) (Date, error) {
	var d Date
	var starts, created string
	var ends sql.NullString
	var mic, cable, stand int
	if err := rs.Scan(&d.ID, &d.Title, &d.Category, &starts, &ends, &d.Location, &d.Notes, &d.Status, &mic, &cable, &stand, &d.Bring.Dress, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Date{}, ErrNotFound
		}
		return Date{}, err
	}
	d.StartsAt = parseTime(starts)
	d.EndsAt = parseTimePtr(ends)
	d.CreatedAt = parseTime(created)
	d.Bring.Mic = mic != 0
	d.Bring.Cable = cable != 0
	d.Bring.Stand = stand != 0
	return d, nil
}

func scanDateRow(row *sql.Row) (Date, error) { return scanDate(row) }

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func slicesContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func normalizeBring(b Bring) (Bring, error) {
	dress, err := NormalizeDress(b.Dress)
	if err != nil {
		return Bring{}, err
	}
	b.Dress = dress
	return b, nil
}
