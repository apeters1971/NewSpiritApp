package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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
	db       *sql.DB
	mediaDir string
}

type User struct {
	ID                 string     `json:"id"`
	Nickname           string     `json:"nickname"`
	Email              string     `json:"email"`
	Role               string     `json:"role"`
	Subrole            string     `json:"subrole"`
	Address            string     `json:"address"`
	Phone              string     `json:"phone"`
	AltEmail           string     `json:"altEmail,omitempty"`
	Birthday           string     `json:"birthday"`
	HasPhoto           bool       `json:"hasPhoto"`
	PhotoUpdatedAt     *time.Time `json:"photoUpdatedAt,omitempty"`
	Channels           []Channel  `json:"channels,omitempty"`
	MustChangePassword bool       `json:"mustChangePassword,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
}

type DirectoryEntry struct {
	ID             string     `json:"id"`
	Nickname       string     `json:"nickname"`
	Email          string     `json:"email"`
	AltEmail       string     `json:"altEmail,omitempty"`
	Phone          string     `json:"phone"`
	Role           string     `json:"role"`
	Subrole        string     `json:"subrole"`
	HasPhoto       bool       `json:"hasPhoto"`
	PhotoUpdatedAt *time.Time `json:"photoUpdatedAt,omitempty"`
}

type Date struct {
	ID             string       `json:"id"`
	Title          string       `json:"title"`
	Category       string       `json:"category"`
	StartsAt       time.Time    `json:"startsAt"`
	EndsAt         *time.Time   `json:"endsAt,omitempty"`
	Location       string       `json:"location,omitempty"`
	Notes          string       `json:"notes,omitempty"`
	Schedule       string       `json:"schedule,omitempty"`
	Status         string       `json:"status"`
	Roles          []string     `json:"roles"`
	Bring          Bring        `json:"bring"`
	FrozenOptionID string       `json:"frozenOptionId,omitempty"`
	PollOpen       bool         `json:"pollOpen"`
	Options        []PollOption `json:"options"`
	CreatedAt      time.Time    `json:"createdAt"`
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
	Attendance    string  `json:"attendance,omitempty"`
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
	ChatOpen      bool           `json:"chatOpen"`
	Titles        []ArchiveItem  `json:"titles"`
	GalleryCount  int            `json:"galleryCount"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(8000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, mediaDir: filepath.Join(filepath.Dir(path), "gallery")}
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
  must_change_password INTEGER NOT NULL DEFAULT 0,
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
  schedule TEXT NOT NULL DEFAULT '',
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
CREATE TABLE IF NOT EXISTS poll_options (
  id TEXT PRIMARY KEY,
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  starts_at TEXT NOT NULL,
  ends_at TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS poll_votes (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  option_id TEXT NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
  choice TEXT NOT NULL,
  initial_choice TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, option_id)
);
CREATE INDEX IF NOT EXISTS idx_poll_options_date ON poll_options(date_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_poll_votes_option ON poll_votes(option_id);
CREATE TABLE IF NOT EXISTS user_photos (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  mime TEXT NOT NULL,
  data BLOB NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS chat_messages (
  id TEXT PRIMARY KEY,
  room TEXT NOT NULL,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  text TEXT NOT NULL,
  created_at TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'text',
  mime TEXT NOT NULL DEFAULT '',
  duration_ms INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_chat_room ON chat_messages(room, created_at);
CREATE TABLE IF NOT EXISTS chat_reactions (
  message_id TEXT NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL DEFAULT '',
  emoji TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (message_id, user_id, emoji)
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN category TEXT NOT NULL DEFAULT 'event'`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN bring_mic INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN bring_cable INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN bring_stand INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN bring_dress TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN frozen_option_id TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN schedule TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN address TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN phone TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN birthday TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN alt_email TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`
CREATE TABLE IF NOT EXISTS chat_reactions (
  message_id TEXT NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL DEFAULT '',
  emoji TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (message_id, user_id, emoji)
)`)
	_, _ = s.db.Exec(`
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
)`)
	if err := s.allowAdminChatMessages(); err != nil {
		return err
	}
	if err := s.migrateArchive(); err != nil {
		return err
	}
	if err := s.migrateChannels(); err != nil {
		return err
	}
	if err := s.migrateProposals(); err != nil {
		return err
	}
	if err := s.migrateCalendar(); err != nil {
		return err
	}
	if err := s.migrateGallery(); err != nil {
		return err
	}
	if err := s.migrateAbsences(); err != nil {
		return err
	}
	if err := s.migrateChatReads(); err != nil {
		return err
	}
	return s.migrateChatVoice()
}

func (s *Store) allowAdminChatMessages() error {
	var notnull int
	err := s.db.QueryRow(`SELECT "notnull" FROM pragma_table_info('chat_messages') WHERE name='user_id'`).Scan(&notnull)
	if err != nil || notnull == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
CREATE TABLE chat_messages_v2 (
  id TEXT PRIMARY KEY,
  room TEXT NOT NULL,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  text TEXT NOT NULL,
  created_at TEXT NOT NULL
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO chat_messages_v2 (id, room, user_id, text, created_at)
SELECT id, room, user_id, text, created_at FROM chat_messages`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE chat_messages`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE chat_messages_v2 RENAME TO chat_messages`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_room ON chat_messages(room, created_at)`); err != nil {
		return err
	}
	return tx.Commit()
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
	u := User{ID: newID(), Nickname: nickname, Email: email, Role: role, Subrole: subrole, MustChangePassword: true, CreatedAt: now()}
	_, err = s.db.Exec(
		`INSERT INTO users(id, nickname, email, password_hash, must_change_password, role, subrole, created_at) VALUES(?,?,?,?,1,?,?,?)`,
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
			`UPDATE users SET nickname=?, email=?, password_hash=?, must_change_password=1, role=?, subrole=? WHERE id=?`,
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
	if !CanHaveChannel(role) {
		if err := s.ClearChannelsForUser(id); err != nil {
			return User{}, err
		}
	}
	return s.UserByID(id)
}

func (s *Store) ChangeOwnPassword(id, password string) (User, error) {
	if len(password) < 6 {
		return User{}, fmt.Errorf("password must be at least 6 characters")
	}
	var hash string
	err := s.db.QueryRow(`SELECT password_hash FROM users WHERE id=?`, id).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil {
		return User{}, fmt.Errorf("choose a different password")
	}
	next, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	if _, err := s.db.Exec(`UPDATE users SET password_hash=?, must_change_password=0 WHERE id=?`, next, id); err != nil {
		return User{}, err
	}
	return s.UserByID(id)
}

func (s *Store) SetUserInfo(id, address, phone, birthday, altEmail string) (User, error) {
	if _, err := s.UserByID(id); err != nil {
		return User{}, err
	}
	address = strings.TrimSpace(spaceRe.ReplaceAllString(address, " "))
	phone = strings.TrimSpace(phone)
	birthday, err := normalizeBirthday(birthday)
	if err != nil {
		return User{}, err
	}
	altEmail, err = normalizeContactEmail(altEmail)
	if err != nil {
		return User{}, err
	}
	if len(address) > 500 {
		return User{}, fmt.Errorf("address is too long")
	}
	if len(phone) > 80 {
		return User{}, fmt.Errorf("phone is too long")
	}
	if _, err := s.db.Exec(`UPDATE users SET address=?, phone=?, birthday=?, alt_email=? WHERE id=?`, address, phone, birthday, altEmail, id); err != nil {
		return User{}, err
	}
	return s.UserByID(id)
}

func normalizeContactEmail(email string) (string, error) {
	email = NormalizeEmail(email)
	if email == "" {
		return "", nil
	}
	if !strings.Contains(email, "@") || utf8.RuneCountInString(email) > 120 {
		return "", fmt.Errorf("invalid email")
	}
	return email, nil
}

func normalizeBirthday(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", fmt.Errorf("invalid birthday")
	}
	return t.Format("2006-01-02"), nil
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
	rows, err := s.db.Query(`SELECT id, nickname, email, role, subrole, address, phone, alt_email, birthday, must_change_password, created_at FROM users ORDER BY nickname COLLATE NOCASE`)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachPhotos(out); err != nil {
		return nil, err
	}
	if err := s.attachChannels(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListDirectory() ([]DirectoryEntry, error) {
	rows, err := s.db.Query(`SELECT id, nickname, email, alt_email, role, subrole, phone FROM users ORDER BY nickname COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DirectoryEntry{}
	for rows.Next() {
		var e DirectoryEntry
		if err := rows.Scan(&e.ID, &e.Nickname, &e.Email, &e.AltEmail, &e.Role, &e.Subrole, &e.Phone); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	times, err := s.photoTimes()
	if err != nil {
		return nil, err
	}
	for i := range out {
		if t, ok := times[out[i].ID]; ok {
			out[i].HasPhoto = true
			tt := t
			out[i].PhotoUpdatedAt = &tt
		}
	}
	return out, nil
}

func (s *Store) UserByID(id string) (User, error) {
	u, err := scanUserRow(s.db.QueryRow(`SELECT id, nickname, email, role, subrole, address, phone, alt_email, birthday, must_change_password, created_at FROM users WHERE id=?`, id))
	if err != nil {
		return User{}, err
	}
	if err := s.attachPhoto(&u); err != nil {
		return User{}, err
	}
	if err := s.attachChannelPtr(&u); err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) Login(email, password string) (User, string, error) {
	email = NormalizeEmail(email)
	if email == "" || password == "" {
		return User{}, "", ErrUnauthorized
	}
	var u User
	var hash, created string
	var mustChange int
	err := s.db.QueryRow(
		`SELECT id, nickname, email, password_hash, role, subrole, address, phone, alt_email, birthday, must_change_password, created_at FROM users WHERE email=?`,
		email,
	).Scan(&u.ID, &u.Nickname, &u.Email, &hash, &u.Role, &u.Subrole, &u.Address, &u.Phone, &u.AltEmail, &u.Birthday, &mustChange, &created)
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
	u.MustChangePassword = mustChange != 0
	sid := newID()
	if _, err := s.db.Exec(`INSERT INTO sessions(id, user_id, created_at) VALUES(?,?,?)`, sid, u.ID, fmtTime(now())); err != nil {
		return User{}, "", err
	}
	if err := s.attachPhoto(&u); err != nil {
		return User{}, "", err
	}
	if err := s.attachChannelPtr(&u); err != nil {
		return User{}, "", err
	}
	return u, sid, nil
}

func (s *Store) UserBySession(sessionID string) (User, error) {
	if sessionID == "" {
		return User{}, ErrUnauthorized
	}
	u, err := scanUserRow(s.db.QueryRow(`
SELECT u.id, u.nickname, u.email, u.role, u.subrole, u.address, u.phone, u.alt_email, u.birthday, u.must_change_password, u.created_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id=? AND s.revoked_at IS NULL`, sessionID))
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	if err := s.attachPhoto(&u); err != nil {
		return User{}, err
	}
	if err := s.attachChannelPtr(&u); err != nil {
		return User{}, err
	}
	return u, nil
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

const dateScheduleMax = 8000

func prepareDateSchedule(text string) (string, error) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if utf8.RuneCountInString(text) > dateScheduleMax {
		return "", fmt.Errorf("schedule is too long")
	}
	return text, nil
}

func (s *Store) CreateDate(title, category string, startsAt time.Time, endsAt *time.Time, location, notes, schedule string, roles []string, bring Bring, options []PollOptionInput) (Date, error) {
	title = NormalizeName(title)
	if title == "" {
		return Date{}, fmt.Errorf("title is required")
	}
	startsAt, endsAt, options, err := resolveSchedule(startsAt, endsAt, options)
	if err != nil {
		return Date{}, err
	}
	category, err = NormalizeCategory(category)
	if err != nil {
		return Date{}, err
	}
	roles, err = NormalizeRoles(roles)
	if err != nil {
		return Date{}, err
	}
	bring, err = normalizeBring(bring)
	if err != nil {
		return Date{}, err
	}
	schedule, err = prepareDateSchedule(schedule)
	if err != nil {
		return Date{}, err
	}
	d := Date{
		ID:        newID(),
		Title:     title,
		Category:  category,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		Location:  NormalizeName(location),
		Notes:     strings.TrimSpace(notes),
		Schedule:  schedule,
		Status:    StatusVoting,
		Roles:     roles,
		Bring:     bring,
		Options:   []PollOption{},
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
		`INSERT INTO dates(id, title, category, starts_at, ends_at, location, notes, schedule, status, bring_mic, bring_cable, bring_stand, bring_dress, frozen_option_id, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.Title, d.Category, fmtTime(d.StartsAt), ends, d.Location, d.Notes, d.Schedule, d.Status, boolInt(d.Bring.Mic), boolInt(d.Bring.Cable), boolInt(d.Bring.Stand), d.Bring.Dress, "", fmtTime(d.CreatedAt),
	); err != nil {
		return Date{}, err
	}
	if err := insertDateRoles(tx, d.ID, d.Roles); err != nil {
		return Date{}, err
	}
	if err := s.replacePollOptions(tx, d.ID, options); err != nil {
		return Date{}, err
	}
	if err := tx.Commit(); err != nil {
		return Date{}, err
	}
	return s.dateRow(d.ID)
}

func (s *Store) UpdateDate(id, title, category string, startsAt time.Time, endsAt *time.Time, location, notes, schedule string, roles []string, bring Bring, options []PollOptionInput) (Date, error) {
	cur, err := s.dateRow(id)
	if err != nil {
		return Date{}, err
	}
	title = NormalizeName(title)
	if title == "" {
		return Date{}, fmt.Errorf("title is required")
	}
	if cur.FrozenOptionID != "" {
		options = nil
		startsAt, endsAt, _, err = resolveSchedule(startsAt, endsAt, nil)
	} else {
		startsAt, endsAt, options, err = resolveSchedule(startsAt, endsAt, options)
	}
	if err != nil {
		return Date{}, err
	}
	category, err = NormalizeCategory(category)
	if err != nil {
		return Date{}, err
	}
	roles, err = NormalizeRoles(roles)
	if err != nil {
		return Date{}, err
	}
	bring, err = normalizeBring(bring)
	if err != nil {
		return Date{}, err
	}
	schedule, err = prepareDateSchedule(schedule)
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
		`UPDATE dates SET title=?, category=?, starts_at=?, ends_at=?, location=?, notes=?, schedule=?, bring_mic=?, bring_cable=?, bring_stand=?, bring_dress=? WHERE id=?`,
		title, category, fmtTime(startsAt.UTC()), ends, NormalizeName(location), strings.TrimSpace(notes), schedule, boolInt(bring.Mic), boolInt(bring.Cable), boolInt(bring.Stand), bring.Dress, id,
	); err != nil {
		return Date{}, err
	}
	if _, err := tx.Exec(`DELETE FROM date_roles WHERE date_id=?`, id); err != nil {
		return Date{}, err
	}
	if err := insertDateRoles(tx, id, roles); err != nil {
		return Date{}, err
	}
	if cur.FrozenOptionID == "" {
		if err := s.replacePollOptions(tx, id, options); err != nil {
			return Date{}, err
		}
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
	_ = s.removeGalleryDir(id)
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
	if status == StatusAccepted && d.PollOpen {
		return Date{}, fmt.Errorf("choose a poll time before accepting")
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
	if d.PollOpen {
		return fmt.Errorf("%w: vote on a poll option instead", ErrForbidden)
	}
	u, err := s.UserByID(userID)
	if err != nil {
		return err
	}
	if !RoleCanVote(u.Role) || !slicesContains(d.Roles, u.Role) {
		return fmt.Errorf("%w: this date is not for your role", ErrForbidden)
	}
	_, err = s.db.Exec(`
INSERT INTO votes(user_id, date_id, choice, initial_choice, updated_at) VALUES(?,?,?,?,?)
ON CONFLICT(user_id, date_id) DO UPDATE SET choice=excluded.choice, updated_at=excluded.updated_at`,
		userID, dateID, choice, choice, fmtTime(now()),
	)
	if err != nil {
		return err
	}
	if choice != VoteYes {
		_, _ = s.db.Exec(`DELETE FROM date_absences WHERE date_id=? AND user_id=?`, dateID, userID)
	}
	return nil
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
	titlesByDate, err := s.titlesForDates([]string{id})
	if err != nil {
		return DateView{}, err
	}
	view, err := s.attachView(d, viewer, commentsByDate[id], titlesByDate[id])
	if err != nil {
		return DateView{}, err
	}
	counts, err := s.galleryCounts([]string{id})
	if err != nil {
		return DateView{}, err
	}
	view.GalleryCount = counts[id]
	return view, nil
}

func (s *Store) ListDateViews(viewer *User) ([]DateView, error) {
	dates, err := s.listDates()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(dates))
	for _, d := range dates {
		if viewer != nil && !RoleSeesDate(viewer.Role, d.Roles) {
			continue
		}
		ids = append(ids, d.ID)
	}
	commentsByDate, err := s.commentsForDates(ids)
	if err != nil {
		return nil, err
	}
	titlesByDate, err := s.titlesForDates(ids)
	if err != nil {
		return nil, err
	}
	galleryCounts, err := s.galleryCounts(ids)
	if err != nil {
		return nil, err
	}
	out := make([]DateView, 0, len(ids))
	for _, d := range dates {
		if viewer != nil && !RoleSeesDate(viewer.Role, d.Roles) {
			continue
		}
		v, err := s.attachView(d, viewer, commentsByDate[d.ID], titlesByDate[d.ID])
		if err != nil {
			return nil, err
		}
		v.GalleryCount = galleryCounts[d.ID]
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) attachView(d Date, viewer *User, comments []Comment, titles []ArchiveItem) (DateView, error) {
	roster, err := s.roster(d)
	if err != nil {
		return DateView{}, err
	}
	if comments == nil {
		comments = []Comment{}
	}
	if titles == nil {
		titles = []ArchiveItem{}
	}
	optionIDs := make([]string, 0, len(d.Options))
	for _, o := range d.Options {
		optionIDs = append(optionIDs, o.ID)
	}
	pollVotes, err := s.pollVotesForOptions(optionIDs)
	if err != nil {
		return DateView{}, err
	}
	d.Options = attachPoll(d.Options, d.FrozenOptionID, roster, pollVotes, viewer)
	d.PollOpen = pollOpen(d.FrozenOptionID, d.Options)
	view := DateView{
		Date:          d,
		MyChoice:      VoteUnknown,
		Roster:        roster,
		SubroleCounts: countsFor(d.Roles, roster),
		Comments:      comments,
		ChatOpen:      EventChatIsOpen(d, now()),
		Titles:        titles,
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
	rows, err := s.db.Query(`SELECT id, title, category, starts_at, ends_at, location, notes, schedule, status, bring_mic, bring_cable, bring_stand, bring_dress, frozen_option_id, created_at FROM dates ORDER BY starts_at`)
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
	if err := s.decorateDates(dates, ids); err != nil {
		return nil, err
	}
	return dates, nil
}

func (s *Store) dateRow(id string) (Date, error) {
	d, err := scanDateRow(s.db.QueryRow(
		`SELECT id, title, category, starts_at, ends_at, location, notes, schedule, status, bring_mic, bring_cable, bring_stand, bring_dress, frozen_option_id, created_at FROM dates WHERE id=?`, id,
	))
	if err != nil {
		return Date{}, err
	}
	dates := []Date{d}
	if err := s.decorateDates(dates, []string{id}); err != nil {
		return Date{}, err
	}
	return dates[0], nil
}

func (s *Store) decorateDates(dates []Date, ids []string) error {
	roleMap, err := s.rolesForDates(ids)
	if err != nil {
		return err
	}
	optMap, err := s.optionsForDates(ids)
	if err != nil {
		return err
	}
	for i := range dates {
		dates[i].Roles = roleMap[dates[i].ID]
		if dates[i].Roles == nil {
			dates[i].Roles = []string{}
		}
		dates[i].Options = optMap[dates[i].ID]
		if dates[i].Options == nil {
			dates[i].Options = []PollOption{}
		}
		dates[i].PollOpen = pollOpen(dates[i].FrozenOptionID, dates[i].Options)
	}
	return nil
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
	args := []any{d.ID, d.ID}
	for _, role := range d.Roles {
		args = append(args, role)
	}
	rows, err := s.db.Query(`
SELECT u.id, u.nickname, u.email, u.role, u.subrole, v.choice, v.initial_choice, a.kind
FROM users u
LEFT JOIN votes v ON v.user_id = u.id AND v.date_id = ?
LEFT JOIN date_absences a ON a.user_id = u.id AND a.date_id = ?
WHERE u.role IN (`+placeholders+`)
ORDER BY u.role, u.subrole, u.nickname COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RosterEntry{}
	for rows.Next() {
		var e RosterEntry
		var choice, initial, kind sql.NullString
		if err := rows.Scan(&e.UserID, &e.Nickname, &e.Email, &e.Role, &e.Subrole, &choice, &initial, &kind); err != nil {
			return nil, err
		}
		e.Attendance = scanAttendance(kind)
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
	var mustChange int
	if err := rs.Scan(&u.ID, &u.Nickname, &u.Email, &u.Role, &u.Subrole, &u.Address, &u.Phone, &u.AltEmail, &u.Birthday, &mustChange, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	u.CreatedAt = parseTime(created)
	u.MustChangePassword = mustChange != 0
	return u, nil
}

func scanUserRow(row *sql.Row) (User, error) { return scanUser(row) }

func scanDate(rs rowScanner) (Date, error) {
	var d Date
	var starts, created string
	var ends sql.NullString
	var mic, cable, stand int
	if err := rs.Scan(&d.ID, &d.Title, &d.Category, &starts, &ends, &d.Location, &d.Notes, &d.Schedule, &d.Status, &mic, &cable, &stand, &d.Bring.Dress, &d.FrozenOptionID, &created); err != nil {
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
