package store

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	GalleryKindPhoto = "photo"
	GalleryKindVideo = "video"
	GalleryMaxPhoto  = 8 << 20
	GalleryMaxVideo  = 1 << 30
)

type GalleryItem struct {
	ID        string `json:"id"`
	DateID    string `json:"dateId"`
	UserID    string `json:"userId,omitempty"`
	Nickname  string `json:"nickname"`
	Kind      string `json:"kind"`
	MIME      string `json:"mime"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

func GalleryMaxBytes() int {
	return GalleryMaxVideo
}

func (s *Store) migrateGallery() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS date_media (
  id TEXT PRIMARY KEY,
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  kind TEXT NOT NULL,
  mime TEXT NOT NULL,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_date_media_date ON date_media(date_id, created_at);
`)
	return err
}

func (s *Store) MemberCanSeeDate(user User, dateID string) error {
	d, err := s.dateRow(dateID)
	if err != nil {
		return err
	}
	if !slicesContains(d.Roles, user.Role) {
		return fmt.Errorf("%w: this date is not for your role", ErrForbidden)
	}
	return nil
}

func (s *Store) galleryCounts(ids []string) (map[string]int, error) {
	out := map[string]int{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT date_id, COUNT(*) FROM date_media WHERE date_id IN (`+placeholders+`) GROUP BY date_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func (s *Store) ListGallery(dateID string) ([]GalleryItem, error) {
	if _, err := s.dateRow(dateID); err != nil {
		return nil, err
	}
	alias := s.AdminAlias()
	rows, err := s.db.Query(`
SELECT m.id, m.date_id, COALESCE(m.user_id, ''), COALESCE(u.nickname, ''), m.kind, m.mime, m.name, m.created_at
FROM date_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.date_id=?
ORDER BY m.created_at, m.id`, dateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GalleryItem{}
	for rows.Next() {
		var item GalleryItem
		if err := rows.Scan(&item.ID, &item.DateID, &item.UserID, &item.Nickname, &item.Kind, &item.MIME, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		if item.Nickname == "" {
			item.Nickname = alias
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) AddGalleryItem(dateID, userID, filename string, data []byte) (GalleryItem, error) {
	return s.AddGalleryItemFromReader(dateID, userID, filename, bytes.NewReader(data))
}

func (s *Store) AddGalleryItemFromReader(dateID, userID, filename string, r io.Reader) (GalleryItem, error) {
	if _, err := s.dateRow(dateID); err != nil {
		return GalleryItem{}, err
	}
	if userID != "" {
		if _, err := s.UserByID(userID); err != nil {
			return GalleryItem{}, err
		}
	}
	if r == nil {
		return GalleryItem{}, fmt.Errorf("file is required")
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		head = head[:n]
	} else if err != nil {
		return GalleryItem{}, fmt.Errorf("file is required")
	} else {
		head = head[:n]
	}
	if len(head) == 0 {
		return GalleryItem{}, fmt.Errorf("file is required")
	}
	kind, mime, err := sniffGallery(filename, head)
	if err != nil {
		return GalleryItem{}, err
	}
	max := GalleryMaxPhoto
	if kind == GalleryKindVideo {
		max = GalleryMaxVideo
	}
	id := newID()
	dir := filepath.Join(s.mediaDir, dateID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return GalleryItem{}, err
	}
	dest := s.galleryPath(dateID, id)
	f, err := os.Create(dest)
	if err != nil {
		return GalleryItem{}, err
	}
	written, copyErr := io.Copy(f, io.MultiReader(bytes.NewReader(head), io.LimitReader(r, int64(max)-int64(len(head))+1)))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return GalleryItem{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return GalleryItem{}, closeErr
	}
	if written > int64(max) {
		_ = os.Remove(dest)
		return GalleryItem{}, fmt.Errorf("file is too large")
	}
	name := sanitizeArchiveName(filename)
	var uid any
	if userID != "" {
		uid = userID
	}
	if _, err := s.db.Exec(
		`INSERT INTO date_media(id, date_id, user_id, kind, mime, name, created_at) VALUES(?,?,?,?,?,?,?)`,
		id, dateID, uid, kind, mime, name, fmtTime(now()),
	); err != nil {
		_ = os.Remove(dest)
		return GalleryItem{}, err
	}
	item, err := s.galleryItemRow(dateID, id)
	if err != nil {
		return GalleryItem{}, err
	}
	return item, nil
}

func (s *Store) GalleryFile(dateID, fileID string) (GalleryItem, []byte, error) {
	item, path, err := s.GalleryFilePath(dateID, fileID)
	if err != nil {
		return GalleryItem{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GalleryItem{}, nil, ErrNotFound
		}
		return GalleryItem{}, nil, err
	}
	return item, data, nil
}

func (s *Store) GalleryFilePath(dateID, fileID string) (GalleryItem, string, error) {
	item, err := s.galleryItemRow(dateID, fileID)
	if err != nil {
		return GalleryItem{}, "", err
	}
	p := s.galleryPath(dateID, fileID)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return GalleryItem{}, "", ErrNotFound
		}
		return GalleryItem{}, "", err
	}
	return item, p, nil
}

func (s *Store) DeleteGalleryItem(dateID, fileID, userID string, asController bool) error {
	item, err := s.galleryItemRow(dateID, fileID)
	if err != nil {
		return err
	}
	if !asController && (userID == "" || item.UserID != userID) {
		return fmt.Errorf("%w: you can only delete your own files", ErrForbidden)
	}
	if _, err := s.db.Exec(`DELETE FROM date_media WHERE id=? AND date_id=?`, fileID, dateID); err != nil {
		return err
	}
	_ = os.Remove(s.galleryPath(dateID, fileID))
	return nil
}

func (s *Store) galleryItemRow(dateID, fileID string) (GalleryItem, error) {
	var item GalleryItem
	var userID sql.NullString
	var nickname sql.NullString
	err := s.db.QueryRow(`
SELECT m.id, m.date_id, m.user_id, u.nickname, m.kind, m.mime, m.name, m.created_at
FROM date_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.id=? AND m.date_id=?`, fileID, dateID,
	).Scan(&item.ID, &item.DateID, &userID, &nickname, &item.Kind, &item.MIME, &item.Name, &item.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return GalleryItem{}, ErrNotFound
		}
		return GalleryItem{}, err
	}
	if userID.Valid {
		item.UserID = userID.String
	}
	if nickname.Valid && nickname.String != "" {
		item.Nickname = nickname.String
	} else {
		item.Nickname = s.AdminAlias()
	}
	return item, nil
}

func (s *Store) galleryPath(dateID, fileID string) string {
	return filepath.Join(s.mediaDir, dateID, fileID)
}

func (s *Store) removeGalleryDir(dateID string) error {
	if dateID == "" || s.mediaDir == "" {
		return nil
	}
	return os.RemoveAll(filepath.Join(s.mediaDir, dateID))
}

func sniffGallery(filename string, data []byte) (kind, mime string, err error) {
	ext := strings.ToLower(path.Ext(filename))
	detected := http.DetectContentType(data)
	if i := strings.IndexByte(detected, ';'); i >= 0 {
		detected = strings.TrimSpace(detected[:i])
	}
	switch {
	case ext == ".jpg" || ext == ".jpeg" || detected == "image/jpeg":
		return GalleryKindPhoto, "image/jpeg", nil
	case ext == ".png" || detected == "image/png":
		return GalleryKindPhoto, "image/png", nil
	case ext == ".webp" || detected == "image/webp":
		return GalleryKindPhoto, "image/webp", nil
	case ext == ".gif" || detected == "image/gif":
		return GalleryKindPhoto, "image/gif", nil
	case ext == ".mp4" || ext == ".m4v" || detected == "video/mp4":
		return GalleryKindVideo, "video/mp4", nil
	case ext == ".webm" || detected == "video/webm":
		return GalleryKindVideo, "video/webm", nil
	case ext == ".mov" || detected == "video/quicktime":
		return GalleryKindVideo, "video/quicktime", nil
	case strings.HasPrefix(detected, "image/"):
		return GalleryKindPhoto, detected, nil
	case strings.HasPrefix(detected, "video/"):
		return GalleryKindVideo, detected, nil
	default:
		if utf8.RuneCountInString(filename) == 0 {
			return "", "", fmt.Errorf("file type is not allowed")
		}
		return "", "", fmt.Errorf("file type is not allowed")
	}
}
