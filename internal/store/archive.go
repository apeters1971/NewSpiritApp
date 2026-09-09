package store

import (
	"database/sql"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ArchiveKindAudio  = "audio"
	ArchiveKindLyrics = "lyrics"
	ArchiveKindSheet  = "sheet"
	ArchiveMaxAudio   = 25 << 20
	ArchiveMaxDoc     = 12 << 20
	archiveTitleMax   = 200
	archiveComposerMax = 120
)

var ArchiveKinds = []string{ArchiveKindAudio, ArchiveKindLyrics, ArchiveKindSheet}

type ArchiveFileMeta struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Role      string    `json:"role,omitempty"`
	MIME      string    `json:"mime"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ArchiveItem struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Composer  string            `json:"composer,omitempty"`
	Files     []ArchiveFileMeta `json:"files"`
	CreatedAt time.Time         `json:"createdAt"`
}

type ArchiveFile struct {
	ArchiveFileMeta
	Data []byte
}

func ValidArchiveKind(kind string) bool {
	return kind == ArchiveKindAudio || kind == ArchiveKindLyrics || kind == ArchiveKindSheet
}

func ArchiveMaxBytes(kind string) int {
	if kind == ArchiveKindAudio {
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
	return s.migrateArchiveFiles()
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

func (s *Store) CreateArchiveItem(title, composer string) (ArchiveItem, error) {
	title, err := prepareArchiveTitle(title)
	if err != nil {
		return ArchiveItem{}, err
	}
	composer, err = prepareArchiveComposer(composer)
	if err != nil {
		return ArchiveItem{}, err
	}
	item := ArchiveItem{
		ID:        newID(),
		Title:     title,
		Composer:  composer,
		Files:     []ArchiveFileMeta{},
		CreatedAt: now(),
	}
	_, err = s.db.Exec(
		`INSERT INTO archive_items(id, title, composer, created_at) VALUES(?,?,?,?)`,
		item.ID, item.Title, item.Composer, fmtTime(item.CreatedAt),
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
		rows, err = s.db.Query(`SELECT id, title, composer, created_at FROM archive_items ORDER BY title COLLATE NOCASE, created_at`)
	} else {
		like := "%" + query + "%"
		rows, err = s.db.Query(
			`SELECT id, title, composer, created_at FROM archive_items WHERE title LIKE ? COLLATE NOCASE OR composer LIKE ? COLLATE NOCASE ORDER BY title COLLATE NOCASE, created_at`,
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
	if _, err := s.archiveItemRow(itemID); err != nil {
		return ArchiveItem{}, err
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
	_, err = s.db.Exec(
		`INSERT INTO archive_files(id, item_id, kind, role, mime, name, data, updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		newID(), itemID, kind, role, mime, name, data, fmtTime(now()),
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
		`SELECT id, kind, role, mime, name, data, updated_at FROM archive_files WHERE id=? AND item_id=?`,
		fileID, itemID,
	).Scan(&f.ID, &f.Kind, &f.Role, &f.MIME, &f.Name, &f.Data, &updated)
	if err == sql.ErrNoRows {
		return ArchiveFile{}, ErrNotFound
	}
	if err != nil {
		return ArchiveFile{}, err
	}
	f.UpdatedAt = parseTime(updated)
	return f, nil
}

func (s *Store) MemberCanAccessArchive(userID, itemID string) error {
	if _, err := s.archiveItemRow(itemID); err != nil {
		return err
	}
	u, err := s.UserByID(userID)
	if err != nil {
		return err
	}
	var n int
	err = s.db.QueryRow(`
SELECT COUNT(1)
FROM date_titles t
JOIN date_roles r ON r.date_id = t.date_id
WHERE t.item_id=? AND r.role=?`, itemID, u.Role).Scan(&n)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: this archive item is not on your dates", ErrForbidden)
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
SELECT t.date_id, a.id, a.title, a.composer, a.created_at
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
		if err := rows.Scan(&dateID, &item.ID, &item.Title, &item.Composer, &created); err != nil {
			return nil, err
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
	row := s.db.QueryRow(`SELECT id, title, composer, created_at FROM archive_items WHERE id=?`, id)
	item, err := scanArchiveItem(row)
	if err == sql.ErrNoRows {
		return ArchiveItem{}, ErrNotFound
	}
	return item, err
}

func scanArchiveItem(rs rowScanner) (ArchiveItem, error) {
	var item ArchiveItem
	var created string
	if err := rs.Scan(&item.ID, &item.Title, &item.Composer, &created); err != nil {
		return ArchiveItem{}, err
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
SELECT id, item_id, kind, role, mime, name, updated_at
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
		if err := rows.Scan(&f.ID, &itemID, &f.Kind, &f.Role, &f.MIME, &f.Name, &updated); err != nil {
			return nil, err
		}
		f.UpdatedAt = parseTime(updated)
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
	case ArchiveKindAudio:
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
	default:
		return "", fmt.Errorf("unknown archive file")
	}
}
