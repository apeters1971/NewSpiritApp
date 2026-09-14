package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ArchiveDropboxItem struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	MIME          string    `json:"mime,omitempty"`
	Kind          string    `json:"kind"`
	Title         string    `json:"title"`
	Author        string    `json:"author"`
	Instrument    string    `json:"instrument"`
	CreatedBy     string    `json:"createdBy,omitempty"`
	CreatedByName string    `json:"createdByName,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	Data          []byte    `json:"-"`
}

func (s *Store) migrateArchiveDropbox() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS archive_dropbox (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  mime TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  instrument TEXT NOT NULL DEFAULT '',
  data BLOB NOT NULL,
  created_by TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_archive_dropbox_created ON archive_dropbox(created_at);
CREATE INDEX IF NOT EXISTS idx_archive_dropbox_user ON archive_dropbox(created_by);
`)
	return err
}

func (s *Store) AddDropboxFile(filename, userID string, data []byte) (ArchiveDropboxItem, error) {
	kind := InferArchiveKind(filename, data)
	if kind == "" {
		return ArchiveDropboxItem{}, fmt.Errorf("file type is not allowed")
	}
	if len(data) == 0 {
		return ArchiveDropboxItem{}, fmt.Errorf("file is required")
	}
	if len(data) > ArchiveMaxBytes(kind) {
		return ArchiveDropboxItem{}, fmt.Errorf("file is too large")
	}
	mime, err := sniffArchiveMIME(kind, filename, data)
	if err != nil {
		return ArchiveDropboxItem{}, err
	}
	name, err := prepareArchiveFileName(filename)
	if err != nil {
		return ArchiveDropboxItem{}, err
	}
	item := ArchiveDropboxItem{
		ID:        newID(),
		Name:      name,
		MIME:      mime,
		Kind:      kind,
		CreatedBy: userID,
		CreatedAt: now(),
		Data:      data,
	}
	_, err = s.db.Exec(
		`INSERT INTO archive_dropbox(id, name, mime, kind, title, author, instrument, data, created_by, created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		item.ID, item.Name, item.MIME, item.Kind, "", "", "", item.Data, item.CreatedBy, fmtTime(item.CreatedAt),
	)
	if err != nil {
		return ArchiveDropboxItem{}, err
	}
	return s.attachDropboxName(item)
}

func (s *Store) ListDropbox(all bool, userID string) ([]ArchiveDropboxItem, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if all {
		rows, err = s.db.Query(`SELECT id, name, mime, kind, title, author, instrument, created_by, created_at FROM archive_dropbox ORDER BY created_at, name COLLATE NOCASE`)
	} else {
		rows, err = s.db.Query(`SELECT id, name, mime, kind, title, author, instrument, created_by, created_at FROM archive_dropbox WHERE created_by=? ORDER BY created_at, name COLLATE NOCASE`, userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArchiveDropboxItem
	for rows.Next() {
		item, err := scanDropboxMeta(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		item, err := s.attachDropboxName(out[i])
		if err != nil {
			return nil, err
		}
		out[i] = item
	}
	if out == nil {
		out = []ArchiveDropboxItem{}
	}
	return out, nil
}

func (s *Store) DropboxItem(id string) (ArchiveDropboxItem, error) {
	item, err := scanDropboxRow(s.db.QueryRow(
		`SELECT id, name, mime, kind, title, author, instrument, data, created_by, created_at FROM archive_dropbox WHERE id=?`,
		id,
	))
	if err != nil {
		return ArchiveDropboxItem{}, err
	}
	return s.attachDropboxName(item)
}

func (s *Store) UpdateDropboxMeta(id, title, author, instrument string) (ArchiveDropboxItem, error) {
	if _, err := s.DropboxItem(id); err != nil {
		return ArchiveDropboxItem{}, err
	}
	title, err := prepareArchiveTitleOptional(title)
	if err != nil {
		return ArchiveDropboxItem{}, err
	}
	author, err = prepareArchiveComposer(author)
	if err != nil {
		return ArchiveDropboxItem{}, err
	}
	instrument, err = prepareArchiveRole(instrument)
	if err != nil {
		return ArchiveDropboxItem{}, err
	}
	if _, err := s.db.Exec(
		`UPDATE archive_dropbox SET title=?, author=?, instrument=? WHERE id=?`,
		title, author, instrument, id,
	); err != nil {
		return ArchiveDropboxItem{}, err
	}
	return s.DropboxItem(id)
}

func (s *Store) DeleteDropbox(id string) error {
	res, err := s.db.Exec(`DELETE FROM archive_dropbox WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ImportDropbox(id, title, author, instrument string) (ArchiveItem, error) {
	drop, err := s.DropboxItem(id)
	if err != nil {
		return ArchiveItem{}, err
	}
	if title == "" {
		title = drop.Title
	}
	if author == "" {
		author = drop.Author
	}
	if instrument == "" {
		instrument = drop.Instrument
	}
	item, err := s.CreateArchiveItem(title, author)
	if err != nil {
		return ArchiveItem{}, err
	}
	item, err = s.AddArchiveFile(item.ID, drop.Kind, instrument, drop.Name, drop.Data)
	if err != nil {
		_, _ = s.db.Exec(`DELETE FROM archive_items WHERE id=?`, item.ID)
		return ArchiveItem{}, err
	}
	if err := s.DeleteDropbox(id); err != nil {
		return ArchiveItem{}, err
	}
	return item, nil
}

func (s *Store) AttachDropbox(id, itemID, instrument string) (ArchiveItem, error) {
	drop, err := s.DropboxItem(id)
	if err != nil {
		return ArchiveItem{}, err
	}
	song, err := s.archiveItemRow(itemID)
	if err != nil {
		return ArchiveItem{}, err
	}
	if song.Status == ArchiveStatusTrashed {
		return ArchiveItem{}, ErrNotFound
	}
	if instrument == "" {
		instrument = drop.Instrument
	}
	item, err := s.AddArchiveFile(song.ID, drop.Kind, instrument, drop.Name, drop.Data)
	if err != nil {
		return ArchiveItem{}, err
	}
	if err := s.DeleteDropbox(id); err != nil {
		return ArchiveItem{}, err
	}
	return item, nil
}

func (s *Store) CanManageDropbox(user User, item ArchiveDropboxItem) bool {
	return user.Archiver || (item.CreatedBy != "" && item.CreatedBy == user.ID)
}

func prepareArchiveTitleOptional(title string) (string, error) {
	title = NormalizeName(title)
	if title == "" {
		return "", nil
	}
	return prepareArchiveTitle(title)
}

func scanDropboxMeta(sc interface {
	Scan(dest ...any) error
}) (ArchiveDropboxItem, error) {
	var item ArchiveDropboxItem
	var created string
	if err := sc.Scan(&item.ID, &item.Name, &item.MIME, &item.Kind, &item.Title, &item.Author, &item.Instrument, &item.CreatedBy, &created); err != nil {
		if err == sql.ErrNoRows {
			return ArchiveDropboxItem{}, ErrNotFound
		}
		return ArchiveDropboxItem{}, err
	}
	item.CreatedAt = parseTime(created)
	return item, nil
}

func scanDropboxRow(sc interface {
	Scan(dest ...any) error
}) (ArchiveDropboxItem, error) {
	var item ArchiveDropboxItem
	var created string
	if err := sc.Scan(&item.ID, &item.Name, &item.MIME, &item.Kind, &item.Title, &item.Author, &item.Instrument, &item.Data, &item.CreatedBy, &created); err != nil {
		if err == sql.ErrNoRows {
			return ArchiveDropboxItem{}, ErrNotFound
		}
		return ArchiveDropboxItem{}, err
	}
	item.CreatedAt = parseTime(created)
	return item, nil
}

func (s *Store) attachDropboxName(item ArchiveDropboxItem) (ArchiveDropboxItem, error) {
	if item.CreatedBy == "" {
		return item, nil
	}
	u, err := s.UserByID(item.CreatedBy)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return item, nil
		}
		return item, err
	}
	item.CreatedByName = u.Nickname
	return item, nil
}
