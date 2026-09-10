package store

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ArchiveKindAudio  = "audio"
	ArchiveKindTracks = "tracks"
	ArchiveKindLyrics = "lyrics"
	ArchiveKindSheet  = "sheet"
	ArchiveKindMIDI   = "midi"
	ArchiveKindLink   = "link"
	ArchiveStatusPending  = "pending"
	ArchiveStatusAccepted = "accepted"
	ArchiveMaxAudio       = 25 << 20
	ArchiveMaxDoc         = 12 << 20
	archiveTitleMax       = 200
	archiveComposerMax    = 120
	archiveURLMax         = 2000
)

var ArchiveKinds = []string{ArchiveKindAudio, ArchiveKindTracks, ArchiveKindLyrics, ArchiveKindSheet, ArchiveKindMIDI, ArchiveKindLink}

type ArchiveFileMeta struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Role      string    `json:"role,omitempty"`
	MIME      string    `json:"mime"`
	Name      string    `json:"name"`
	URL       string    `json:"url,omitempty"`
	Status    string    `json:"status"`
	CreatedBy string    `json:"createdBy,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ArchiveItem struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Composer  string            `json:"composer,omitempty"`
	Status    string            `json:"status"`
	CreatedBy string            `json:"createdBy,omitempty"`
	Files     []ArchiveFileMeta `json:"files"`
	CreatedAt time.Time         `json:"createdAt"`
}

type ArchiveFile struct {
	ArchiveFileMeta
	Data []byte
}

func ValidArchiveKind(kind string) bool {
	for _, k := range ArchiveKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func ArchiveKindIsAudio(kind string) bool {
	return kind == ArchiveKindAudio || kind == ArchiveKindTracks
}

func ArchiveMaxBytes(kind string) int {
	if ArchiveKindIsAudio(kind) {
		return ArchiveMaxAudio
	}
	return ArchiveMaxDoc
}

func (s *Store) migrateArchive() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS archive_items (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  composer TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS date_titles (
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  item_id TEXT NOT NULL REFERENCES archive_items(id) ON DELETE CASCADE,
  sort_order INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (date_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_archive_title ON archive_items(title);
CREATE INDEX IF NOT EXISTS idx_date_titles_date ON date_titles(date_id, sort_order);
`)
	if err != nil {
		return err
	}
	if err := s.migrateArchiveFiles(); err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE archive_items ADD COLUMN status TEXT NOT NULL DEFAULT 'accepted'`)
	_, _ = s.db.Exec(`ALTER TABLE archive_items ADD COLUMN created_by TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE archive_files ADD COLUMN status TEXT NOT NULL DEFAULT 'accepted'`)
	_, _ = s.db.Exec(`ALTER TABLE archive_files ADD COLUMN created_by TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE archive_files ADD COLUMN url TEXT NOT NULL DEFAULT ''`)
	return nil
}

func archiveFilesDDL() string {
	return `CREATE TABLE archive_files (
  id TEXT PRIMARY KEY,
  item_id TEXT NOT NULL REFERENCES archive_items(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT '',
  mime TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  data BLOB NOT NULL,
  updated_at TEXT NOT NULL
)`
}

func (s *Store) migrateArchiveFiles() error {
	var ddl string
	err := s.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='archive_files'`).Scan(&ddl)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec(archiveFilesDDL())
		return err
	}
	if err != nil {
		return err
	}
	if !strings.Contains(strings.ToUpper(ddl), "UNIQUE") {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`CREATE TABLE archive_files_v2 (
  id TEXT PRIMARY KEY,
  item_id TEXT NOT NULL REFERENCES archive_items(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT '',
  mime TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  data BLOB NOT NULL,
  updated_at TEXT NOT NULL
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO archive_files_v2(id, item_id, kind, role, mime, name, data, updated_at)
SELECT id, item_id, kind, role, mime, name, data, updated_at FROM archive_files`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE archive_files`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE archive_files_v2 RENAME TO archive_files`); err != nil {
		return err
	}
	return tx.Commit()
}

func prepareArchiveTitle(title string) (string, error) {
	title = NormalizeName(title)
	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	if utf8.RuneCountInString(title) > archiveTitleMax {
		return "", fmt.Errorf("title is too long")
	}
	return title, nil
}

func prepareArchiveComposer(composer string) (string, error) {
	composer = NormalizeName(composer)
	if utf8.RuneCountInString(composer) > archiveComposerMax {
		return "", fmt.Errorf("composer is too long")
	}
	return composer, nil
}

func prepareArchiveRole(role string) (string, error) {
	role = NormalizeName(role)
	if utf8.RuneCountInString(role) > 40 {
		return "", fmt.Errorf("role is too long")
	}
	return role, nil
}

func prepareArchiveFileName(name string) (string, error) {
	name = sanitizeArchiveName(name)
	if name == "" {
		return "", fmt.Errorf("file name is required")
	}
	return name, nil
}

func prepareArchiveLabel(name, fallback string) (string, error) {
	name = NormalizeName(name)
	if name == "" {
		name = NormalizeName(fallback)
	}
	if name == "" {
		name = "link"
	}
	if utf8.RuneCountInString(name) > 120 {
		return "", fmt.Errorf("file name is too long")
	}
	return name, nil
}

func prepareArchiveURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("url is required")
	}
	if utf8.RuneCountInString(raw) > archiveURLMax {
		return "", fmt.Errorf("url is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("url is invalid")
	}
	return raw, nil
}

func linkLabelFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

func (s *Store) CreateArchiveItem(title, composer string) (ArchiveItem, error) {
	return s.createArchiveItem(title, composer, "", ArchiveStatusAccepted)
}

func (s *Store) CreateMemberArchiveItem(title, composer, userID string) (ArchiveItem, error) {
	return s.createArchiveItem(title, composer, userID, ArchiveStatusPending)
}

func (s *Store) createArchiveItem(title, composer, createdBy, status string) (ArchiveItem, error) {
	title, err := prepareArchiveTitle(title)
	if err != nil {
		return ArchiveItem{}, err
	}
	composer, err = prepareArchiveComposer(composer)
	if err != nil {
		return ArchiveItem{}, err
	}
	if status != ArchiveStatusPending {
		status = ArchiveStatusAccepted
	}
	item := ArchiveItem{
		ID:        newID(),
		Title:     title,
		Composer:  composer,
		Status:    status,
		CreatedBy: createdBy,
		Files:     []ArchiveFileMeta{},
		CreatedAt: now(),
	}
	_, err = s.db.Exec(
		`INSERT INTO archive_items(id, title, composer, status, created_by, created_at) VALUES(?,?,?,?,?,?)`,
		item.ID, item.Title, item.Composer, item.Status, item.CreatedBy, fmtTime(item.CreatedAt),
	)
	if err != nil {
		return ArchiveItem{}, err
	}
	return item, nil
}

func (s *Store) UpdateArchiveItem(id, title, composer string) (ArchiveItem, error) {
	if _, err := s.archiveItemRow(id); err != nil {
		return ArchiveItem{}, err
	}
	title, err := prepareArchiveTitle(title)
	if err != nil {
		return ArchiveItem{}, err
	}
	composer, err = prepareArchiveComposer(composer)
	if err != nil {
		return ArchiveItem{}, err
	}
	if _, err := s.db.Exec(`UPDATE archive_items SET title=?, composer=? WHERE id=?`, title, composer, id); err != nil {
		return ArchiveItem{}, err
	}
	return s.ArchiveItem(id)
}

func (s *Store) DeleteArchiveItem(id string) error {
	res, err := s.db.Exec(`DELETE FROM archive_items WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ArchiveItem(id string) (ArchiveItem, error) {
	item, err := s.archiveItemRow(id)
	if err != nil {
		return ArchiveItem{}, err
	}
	files, err := s.archiveFileMetas([]string{id})
	if err != nil {
		return ArchiveItem{}, err
	}
	item.Files = files[id]
	if item.Files == nil {
		item.Files = []ArchiveFileMeta{}
	}
	return item, nil
}

func (s *Store) ListArchive(query string) ([]ArchiveItem, error) {
	query = strings.TrimSpace(query)
	var rows *sql.Rows
	var err error
	if query == "" {
		rows, err = s.db.Query(`SELECT id, title, composer, status, created_by, created_at FROM archive_items ORDER BY title COLLATE NOCASE, created_at`)
	} else {
		like := "%" + query + "%"
		rows, err = s.db.Query(
			`SELECT id, title, composer, status, created_by, created_at FROM archive_items WHERE title LIKE ? COLLATE NOCASE OR composer LIKE ? COLLATE NOCASE ORDER BY title COLLATE NOCASE, created_at`,
			like, like,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ArchiveItem{}
	ids := []string{}
	for rows.Next() {
		item, err := scanArchiveItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	files, err := s.archiveFileMetas(ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Files = files[items[i].ID]
		if items[i].Files == nil {
			items[i].Files = []ArchiveFileMeta{}
		}
	}
	return items, nil
}

func (s *Store) AddArchiveFile(itemID, kind, role, filename string, data []byte) (ArchiveItem, error) {
	return s.addArchiveFile(itemID, kind, role, filename, data, "", ArchiveStatusAccepted)
}

func (s *Store) AddMemberArchiveFile(itemID, kind, role, filename, userID string, data []byte) (ArchiveItem, error) {
	return s.addArchiveFile(itemID, kind, role, filename, data, userID, ArchiveStatusPending)
}

func (s *Store) AddArchiveLink(itemID, name, rawURL string) (ArchiveItem, error) {
	return s.addArchiveLink(itemID, name, rawURL, "", ArchiveStatusAccepted)
}

func (s *Store) AddMemberArchiveLink(itemID, name, rawURL, userID string) (ArchiveItem, error) {
	return s.addArchiveLink(itemID, name, rawURL, userID, ArchiveStatusPending)
}

func (s *Store) addArchiveLink(itemID, name, rawURL, createdBy, status string) (ArchiveItem, error) {
	if _, err := s.archiveItemRow(itemID); err != nil {
		return ArchiveItem{}, err
	}
	rawURL, err := prepareArchiveURL(rawURL)
	if err != nil {
		return ArchiveItem{}, err
	}
	name, err = prepareArchiveLabel(name, linkLabelFromURL(rawURL))
	if err != nil {
		return ArchiveItem{}, err
	}
	if status != ArchiveStatusPending {
		status = ArchiveStatusAccepted
	}
	_, err = s.db.Exec(
		`INSERT INTO archive_files(id, item_id, kind, role, mime, name, data, status, created_by, updated_at, url) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		newID(), itemID, ArchiveKindLink, "", "text/uri-list", name, []byte(rawURL), status, createdBy, fmtTime(now()), rawURL,
	)
	if err != nil {
		return ArchiveItem{}, err
	}
	return s.ArchiveItem(itemID)
}

func (s *Store) addArchiveFile(itemID, kind, role, filename string, data []byte, createdBy, status string) (ArchiveItem, error) {
	if _, err := s.archiveItemRow(itemID); err != nil {
		return ArchiveItem{}, err
	}
	if kind == ArchiveKindLink {
		return ArchiveItem{}, fmt.Errorf("url is required")
	}
	if !ValidArchiveKind(kind) {
		return ArchiveItem{}, fmt.Errorf("unknown archive file")
	}
	role, err := prepareArchiveRole(role)
	if err != nil {
		return ArchiveItem{}, err
	}
	if len(data) == 0 {
		return ArchiveItem{}, fmt.Errorf("file is required")
	}
	if len(data) > ArchiveMaxBytes(kind) {
		return ArchiveItem{}, fmt.Errorf("file is too large")
	}
	mime, err := sniffArchiveMIME(kind, filename, data)
	if err != nil {
		return ArchiveItem{}, err
	}
	name, err := prepareArchiveFileName(filename)
	if err != nil {
		return ArchiveItem{}, err
	}
	if status != ArchiveStatusPending {
		status = ArchiveStatusAccepted
	}
	_, err = s.db.Exec(
		`INSERT INTO archive_files(id, item_id, kind, role, mime, name, data, status, created_by, updated_at, url) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		newID(), itemID, kind, role, mime, name, data, status, createdBy, fmtTime(now()), "",
	)
	if err != nil {
		return ArchiveItem{}, err
	}
	return s.ArchiveItem(itemID)
}

func (s *Store) UpdateArchiveFile(itemID, fileID, name, role string) (ArchiveItem, error) {
	if _, err := s.GetArchiveFile(itemID, fileID); err != nil {
		return ArchiveItem{}, err
	}
	name, err := prepareArchiveFileName(name)
	if err != nil {
		return ArchiveItem{}, err
	}
	role, err = prepareArchiveRole(role)
	if err != nil {
		return ArchiveItem{}, err
	}
	if _, err := s.db.Exec(
		`UPDATE archive_files SET name=?, role=?, updated_at=? WHERE id=? AND item_id=?`,
		name, role, fmtTime(now()), fileID, itemID,
	); err != nil {
		return ArchiveItem{}, err
	}
	return s.ArchiveItem(itemID)
}

func (s *Store) UpdateArchiveLink(itemID, fileID, name, rawURL string) (ArchiveItem, error) {
	f, err := s.GetArchiveFile(itemID, fileID)
	if err != nil {
		return ArchiveItem{}, err
	}
	if f.Kind != ArchiveKindLink {
		return ArchiveItem{}, fmt.Errorf("unknown archive file")
	}
	rawURL, err = prepareArchiveURL(rawURL)
	if err != nil {
		return ArchiveItem{}, err
	}
	name, err = prepareArchiveLabel(name, linkLabelFromURL(rawURL))
	if err != nil {
		return ArchiveItem{}, err
	}
	if _, err := s.db.Exec(
		`UPDATE archive_files SET name=?, url=?, data=?, updated_at=? WHERE id=? AND item_id=?`,
		name, rawURL, []byte(rawURL), fmtTime(now()), fileID, itemID,
	); err != nil {
		return ArchiveItem{}, err
	}
	return s.ArchiveItem(itemID)
}

func (s *Store) DeleteArchiveFile(itemID, fileID string) (ArchiveItem, error) {
	if _, err := s.archiveItemRow(itemID); err != nil {
		return ArchiveItem{}, err
	}
	res, err := s.db.Exec(`DELETE FROM archive_files WHERE id=? AND item_id=?`, fileID, itemID)
	if err != nil {
		return ArchiveItem{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ArchiveItem{}, ErrNotFound
	}
	return s.ArchiveItem(itemID)
}

func (s *Store) GetArchiveFile(itemID, fileID string) (ArchiveFile, error) {
	var f ArchiveFile
	var updated string
	err := s.db.QueryRow(
		`SELECT id, kind, role, mime, name, data, status, created_by, updated_at, url FROM archive_files WHERE id=? AND item_id=?`,
		fileID, itemID,
	).Scan(&f.ID, &f.Kind, &f.Role, &f.MIME, &f.Name, &f.Data, &f.Status, &f.CreatedBy, &updated, &f.URL)
	if err == sql.ErrNoRows {
		return ArchiveFile{}, ErrNotFound
	}
	if err != nil {
		return ArchiveFile{}, err
	}
	if f.Status == "" {
		f.Status = ArchiveStatusAccepted
	}
	if f.Kind == ArchiveKindLink && f.URL == "" {
		f.URL = strings.TrimSpace(string(f.Data))
	}
	f.UpdatedAt = parseTime(updated)
	return f, nil
}

func (s *Store) MemberCanDownloadArchiveFile(userID, itemID, fileID string) error {
	if err := s.MemberCanAccessArchive(userID, itemID); err != nil {
		return err
	}
	item, err := s.archiveItemRow(itemID)
	if err != nil {
		return err
	}
	f, err := s.GetArchiveFile(itemID, fileID)
	if err != nil {
		return err
	}
	if f.CreatedBy != "" && f.CreatedBy == userID {
		return nil
	}
	if item.Status == ArchiveStatusAccepted && f.Status == ArchiveStatusAccepted {
		return nil
	}
	return fmt.Errorf("%w: this archive file is pending", ErrForbidden)
}

func (s *Store) AcceptArchiveItem(id string) (ArchiveItem, error) {
	if _, err := s.archiveItemRow(id); err != nil {
		return ArchiveItem{}, err
	}
	if _, err := s.db.Exec(`UPDATE archive_items SET status=? WHERE id=?`, ArchiveStatusAccepted, id); err != nil {
		return ArchiveItem{}, err
	}
	return s.ArchiveItem(id)
}

func (s *Store) AcceptArchiveFile(itemID, fileID string) (ArchiveItem, error) {
	if _, err := s.GetArchiveFile(itemID, fileID); err != nil {
		return ArchiveItem{}, err
	}
	if _, err := s.db.Exec(`UPDATE archive_files SET status=?, updated_at=? WHERE id=? AND item_id=?`, ArchiveStatusAccepted, fmtTime(now()), fileID, itemID); err != nil {
		return ArchiveItem{}, err
	}
	return s.ArchiveItem(itemID)
}

func (s *Store) MemberCanAccessArchive(userID, itemID string) error {
	if _, err := s.UserByID(userID); err != nil {
		return err
	}
	if _, err := s.archiveItemRow(itemID); err != nil {
		return err
	}
	return nil
}

func (s *Store) SetDateTitles(dateID string, itemIDs []string) error {
	if _, err := s.dateRow(dateID); err != nil {
		return err
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(itemIDs))
	for _, id := range itemIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		if _, err := s.archiveItemRow(id); err != nil {
			return err
		}
		seen[id] = true
		clean = append(clean, id)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM date_titles WHERE date_id=?`, dateID); err != nil {
		return err
	}
	for i, id := range clean {
		if _, err := tx.Exec(`INSERT INTO date_titles(date_id, item_id, sort_order) VALUES(?,?,?)`, dateID, id, i); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) titlesForDates(ids []string) (map[string][]ArchiveItem, error) {
	out := map[string][]ArchiveItem{}
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
SELECT t.date_id, a.id, a.title, a.composer, a.status, a.created_by, a.created_at
FROM date_titles t
JOIN archive_items a ON a.id = t.item_id
WHERE t.date_id IN (`+placeholders+`)
ORDER BY t.sort_order, a.title COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	itemIDs := []string{}
	seenItem := map[string]bool{}
	type pair struct {
		dateID string
		item   ArchiveItem
	}
	pairs := []pair{}
	for rows.Next() {
		var dateID, created string
		var item ArchiveItem
		if err := rows.Scan(&dateID, &item.ID, &item.Title, &item.Composer, &item.Status, &item.CreatedBy, &created); err != nil {
			return nil, err
		}
		if item.Status == "" {
			item.Status = ArchiveStatusAccepted
		}
		item.CreatedAt = parseTime(created)
		item.Files = []ArchiveFileMeta{}
		pairs = append(pairs, pair{dateID, item})
		if !seenItem[item.ID] {
			seenItem[item.ID] = true
			itemIDs = append(itemIDs, item.ID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	files, err := s.archiveFileMetas(itemIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range pairs {
		item := p.item
		item.Files = files[item.ID]
		if item.Files == nil {
			item.Files = []ArchiveFileMeta{}
		}
		out[p.dateID] = append(out[p.dateID], item)
	}
	return out, nil
}

func (s *Store) archiveItemRow(id string) (ArchiveItem, error) {
	row := s.db.QueryRow(`SELECT id, title, composer, status, created_by, created_at FROM archive_items WHERE id=?`, id)
	item, err := scanArchiveItem(row)
	if err == sql.ErrNoRows {
		return ArchiveItem{}, ErrNotFound
	}
	return item, err
}

func scanArchiveItem(rs rowScanner) (ArchiveItem, error) {
	var item ArchiveItem
	var created string
	if err := rs.Scan(&item.ID, &item.Title, &item.Composer, &item.Status, &item.CreatedBy, &created); err != nil {
		return ArchiveItem{}, err
	}
	if item.Status == "" {
		item.Status = ArchiveStatusAccepted
	}
	item.CreatedAt = parseTime(created)
	item.Files = []ArchiveFileMeta{}
	return item, nil
}

func (s *Store) archiveFileMetas(ids []string) (map[string][]ArchiveFileMeta, error) {
	out := map[string][]ArchiveFileMeta{}
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
SELECT id, item_id, kind, role, mime, name, status, created_by, updated_at, url
FROM archive_files
WHERE item_id IN (`+placeholders+`)
ORDER BY kind, role COLLATE NOCASE, name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID, updated string
		var f ArchiveFileMeta
		if err := rows.Scan(&f.ID, &itemID, &f.Kind, &f.Role, &f.MIME, &f.Name, &f.Status, &f.CreatedBy, &updated, &f.URL); err != nil {
			return nil, err
		}
		if f.Status == "" {
			f.Status = ArchiveStatusAccepted
		}
		f.UpdatedAt = parseTime(updated)
		if f.Kind != ArchiveKindLink {
			f.URL = ""
		}
		out[itemID] = append(out[itemID], f)
	}
	return out, rows.Err()
}

func sanitizeArchiveName(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	if utf8.RuneCountInString(name) > 120 {
		runes := []rune(name)
		name = string(runes[:120])
	}
	return name
}

func sniffArchiveMIME(kind, filename string, data []byte) (string, error) {
	ext := strings.ToLower(path.Ext(filename))
	detected := http.DetectContentType(data)
	if i := strings.IndexByte(detected, ';'); i >= 0 {
		detected = strings.TrimSpace(detected[:i])
	}
	switch kind {
	case ArchiveKindAudio, ArchiveKindTracks:
		switch {
		case ext == ".mp3" || detected == "audio/mpeg":
			return "audio/mpeg", nil
		case ext == ".wav" || detected == "audio/wav" || detected == "audio/x-wav":
			return "audio/wav", nil
		case ext == ".ogg" || detected == "audio/ogg":
			return "audio/ogg", nil
		case ext == ".m4a" || ext == ".mp4" || detected == "audio/mp4":
			return "audio/mp4", nil
		case ext == ".aac":
			return "audio/aac", nil
		case strings.HasPrefix(detected, "audio/"):
			return detected, nil
		}
		return "", fmt.Errorf("file type is not allowed")
	case ArchiveKindLyrics, ArchiveKindSheet:
		switch {
		case ext == ".pdf" || detected == "application/pdf":
			return "application/pdf", nil
		case ext == ".txt" || strings.HasPrefix(detected, "text/plain"):
			return "text/plain; charset=utf-8", nil
		case ext == ".png" || detected == "image/png":
			return "image/png", nil
		case ext == ".jpg" || ext == ".jpeg" || detected == "image/jpeg":
			return "image/jpeg", nil
		case ext == ".webp" || detected == "image/webp":
			return "image/webp", nil
		}
		return "", fmt.Errorf("file type is not allowed")
	case ArchiveKindMIDI:
		return sniffArchiveMIDI(ext, data)
	default:
		return "", fmt.Errorf("unknown archive file")
	}
}

func sniffArchiveMIDI(ext string, data []byte) (string, error) {
	if isMIDIData(data) && (ext == "" || ext == ".mid" || ext == ".midi" || ext == ".kar") {
		return "audio/midi", nil
	}
	if isMXLData(data) && ext == ".mxl" {
		return "application/vnd.recordare.musicxml", nil
	}
	if isMusicXMLData(data) && (ext == "" || ext == ".xml" || ext == ".musicxml") {
		return "application/vnd.recordare.musicxml+xml", nil
	}
	return "", fmt.Errorf("file type is not allowed")
}

func isMIDIData(data []byte) bool {
	return len(data) >= 8 && string(data[:4]) == "MThd"
}

func isMXLData(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && (data[2] == 3 || data[2] == 5 || data[2] == 7)
}

func isMusicXMLData(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	head := strings.ToLower(string(data[:n]))
	return strings.Contains(head, "score-partwise") || strings.Contains(head, "score-timewise") || strings.Contains(head, "musicxml.org") || strings.Contains(head, "recordare")
}
