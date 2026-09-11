package store

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	AlbumLive     = "live"
	AlbumGeneral  = "general"
	AlbumKindDate = "date"
)

type GalleryAlbum struct {
	ID       string        `json:"id"`
	Kind     string        `json:"kind"`
	Title    string        `json:"title"`
	StartsAt *time.Time    `json:"startsAt,omitempty"`
	Category string        `json:"category,omitempty"`
	Count    int           `json:"count"`
	Cover    *GalleryItem  `json:"cover,omitempty"`
	Items    []GalleryItem `json:"items,omitempty"`
}

type AlbumMonth struct {
	Year  int          `json:"year"`
	Month int          `json:"month"`
	Count int          `json:"count"`
	Cover *GalleryItem `json:"cover,omitempty"`
}

func ValidAlbum(album string) bool {
	return album == AlbumLive || album == AlbumGeneral
}

func (s *Store) migrateAlbums() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS album_media (
  id TEXT PRIMARY KEY,
  album TEXT NOT NULL,
  user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  kind TEXT NOT NULL,
  mime TEXT NOT NULL,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_album_media_album ON album_media(album, created_at);
`)
	return err
}

func (s *Store) ListAlbum(album string) ([]GalleryItem, error) {
	if !ValidAlbum(album) {
		return nil, ErrNotFound
	}
	alias := s.AdminAlias()
	rows, err := s.db.Query(`
SELECT m.id, m.album, COALESCE(m.user_id, ''), COALESCE(u.nickname, ''), m.kind, m.mime, m.name, m.created_at
FROM album_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.album=?
ORDER BY m.created_at, m.id`, album)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GalleryItem{}
	for rows.Next() {
		var item GalleryItem
		if err := rows.Scan(&item.ID, &item.Album, &item.UserID, &item.Nickname, &item.Kind, &item.MIME, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		if item.Nickname == "" {
			item.Nickname = alias
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func validYearMonth(year, month int) error {
	if year < 1970 || year > 2100 || month < 1 || month > 12 {
		return fmt.Errorf("invalid month")
	}
	return nil
}

func albumMonthPrefix(year, month int) string {
	return fmt.Sprintf("%04d-%02d-", year, month)
}

func (s *Store) ListAlbumMonths(album string) ([]AlbumMonth, error) {
	if !ValidAlbum(album) {
		return nil, ErrNotFound
	}
	rows, err := s.db.Query(`
SELECT substr(created_at, 1, 4), substr(created_at, 6, 2), COUNT(*)
FROM album_media
WHERE album=?
GROUP BY 1, 2
ORDER BY 1 DESC, 2 DESC`, album)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlbumMonth{}
	for rows.Next() {
		var yearS, monthS string
		var n int
		if err := rows.Scan(&yearS, &monthS, &n); err != nil {
			return nil, err
		}
		var year, month int
		if _, err := fmt.Sscanf(yearS, "%d", &year); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(monthS, "%d", &month); err != nil {
			continue
		}
		if err := validYearMonth(year, month); err != nil {
			continue
		}
		out = append(out, AlbumMonth{Year: year, Month: month, Count: n})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		cover, err := s.albumMonthCover(album, out[i].Year, out[i].Month)
		if err != nil && err != ErrNotFound {
			return nil, err
		}
		if err == nil {
			out[i].Cover = &cover
		}
	}
	return out, nil
}

func (s *Store) ListAlbumMonth(album string, year, month int) ([]GalleryItem, error) {
	if !ValidAlbum(album) {
		return nil, ErrNotFound
	}
	if err := validYearMonth(year, month); err != nil {
		return nil, err
	}
	alias := s.AdminAlias()
	rows, err := s.db.Query(`
SELECT m.id, m.album, COALESCE(m.user_id, ''), COALESCE(u.nickname, ''), m.kind, m.mime, m.name, m.created_at
FROM album_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.album=? AND m.created_at LIKE ?
ORDER BY m.created_at, m.id`, album, albumMonthPrefix(year, month)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GalleryItem{}
	for rows.Next() {
		var item GalleryItem
		if err := rows.Scan(&item.ID, &item.Album, &item.UserID, &item.Nickname, &item.Kind, &item.MIME, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		if item.Nickname == "" {
			item.Nickname = alias
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) albumMonthCover(album string, year, month int) (GalleryItem, error) {
	like := albumMonthPrefix(year, month) + "%"
	item, err := s.scanAlbumCover(`
SELECT m.id, m.album, COALESCE(m.user_id, ''), COALESCE(u.nickname, ''), m.kind, m.mime, m.name, m.created_at
FROM album_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.album=? AND m.kind=? AND m.created_at LIKE ?
ORDER BY m.created_at DESC, m.id DESC LIMIT 1`, album, GalleryKindPhoto, like)
	if err == nil || err != ErrNotFound {
		return item, err
	}
	return s.scanAlbumCover(`
SELECT m.id, m.album, COALESCE(m.user_id, ''), COALESCE(u.nickname, ''), m.kind, m.mime, m.name, m.created_at
FROM album_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.album=? AND m.created_at LIKE ?
ORDER BY m.created_at DESC, m.id DESC LIMIT 1`, album, like)
}

func (s *Store) AddAlbumItem(album, userID, filename string, data []byte) (GalleryItem, error) {
	return s.AddAlbumItemFromReader(album, userID, filename, bytes.NewReader(data))
}

func (s *Store) AddAlbumItemFromReader(album, userID, filename string, r io.Reader) (GalleryItem, error) {
	if !ValidAlbum(album) {
		return GalleryItem{}, ErrNotFound
	}
	if _, err := s.UserByID(userID); err != nil {
		return GalleryItem{}, err
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
	if album == AlbumGeneral && kind == GalleryKindVideo {
		return GalleryItem{}, fmt.Errorf("file type is not allowed")
	}
	max := GalleryMaxPhoto
	if kind == GalleryKindVideo {
		max = GalleryMaxVideo
	}
	id := newID()
	dir := filepath.Join(s.mediaDir, "albums", album)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return GalleryItem{}, err
	}
	dest := s.albumPath(album, id)
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
	if _, err := s.db.Exec(
		`INSERT INTO album_media(id, album, user_id, kind, mime, name, created_at) VALUES(?,?,?,?,?,?,?)`,
		id, album, userID, kind, mime, name, fmtTime(now()),
	); err != nil {
		_ = os.Remove(dest)
		return GalleryItem{}, err
	}
	return s.albumItemRow(album, id)
}

func (s *Store) AlbumFilePath(album, fileID string) (GalleryItem, string, error) {
	item, err := s.albumItemRow(album, fileID)
	if err != nil {
		return GalleryItem{}, "", err
	}
	p := s.albumPath(album, fileID)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return GalleryItem{}, "", ErrNotFound
		}
		return GalleryItem{}, "", err
	}
	return item, p, nil
}

func (s *Store) ListGalleryHub(viewer User) ([]GalleryAlbum, error) {
	live, err := s.albumSummary(AlbumLive, "Live")
	if err != nil {
		return nil, err
	}
	general, err := s.albumSummary(AlbumGeneral, "General")
	if err != nil {
		return nil, err
	}
	albums := []GalleryAlbum{live, general}
	dates, err := s.listDates()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(dates))
	visible := make([]Date, 0, len(dates))
	for _, d := range dates {
		if !RoleSeesDate(viewer.Role, d.Roles) {
			continue
		}
		visible = append(visible, d)
		ids = append(ids, d.ID)
	}
	counts, err := s.galleryCounts(ids)
	if err != nil {
		return nil, err
	}
	for i := len(visible) - 1; i >= 0; i-- {
		d := visible[i]
		n := counts[d.ID]
		if n == 0 {
			continue
		}
		cover, err := s.galleryCover(d.ID)
		if err != nil {
			return nil, err
		}
		start := d.StartsAt
		albums = append(albums, GalleryAlbum{
			ID:       d.ID,
			Kind:     AlbumKindDate,
			Title:    d.Title,
			StartsAt: &start,
			Category: d.Category,
			Count:    n,
			Cover:    &cover,
		})
	}
	return albums, nil
}

func (s *Store) albumSummary(album, title string) (GalleryAlbum, error) {
	count, err := s.albumCount(album)
	if err != nil {
		return GalleryAlbum{}, err
	}
	out := GalleryAlbum{ID: album, Kind: album, Title: title, Count: count}
	if count == 0 {
		return out, nil
	}
	cover, err := s.albumCover(album)
	if err != nil {
		return GalleryAlbum{}, err
	}
	out.Cover = &cover
	return out, nil
}

func (s *Store) albumCount(album string) (int, error) {
	if !ValidAlbum(album) {
		return 0, ErrNotFound
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM album_media WHERE album=?`, album).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) albumCover(album string) (GalleryItem, error) {
	item, err := s.scanAlbumCover(`
SELECT m.id, m.album, COALESCE(m.user_id, ''), COALESCE(u.nickname, ''), m.kind, m.mime, m.name, m.created_at
FROM album_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.album=? AND m.kind=?
ORDER BY m.created_at DESC, m.id DESC LIMIT 1`, album, GalleryKindPhoto)
	if err == nil || err != ErrNotFound {
		return item, err
	}
	return s.scanAlbumCover(`
SELECT m.id, m.album, COALESCE(m.user_id, ''), COALESCE(u.nickname, ''), m.kind, m.mime, m.name, m.created_at
FROM album_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.album=?
ORDER BY m.created_at DESC, m.id DESC LIMIT 1`, album)
}

func (s *Store) scanAlbumCover(query string, args ...any) (GalleryItem, error) {
	var item GalleryItem
	err := s.db.QueryRow(query, args...).Scan(&item.ID, &item.Album, &item.UserID, &item.Nickname, &item.Kind, &item.MIME, &item.Name, &item.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return GalleryItem{}, ErrNotFound
		}
		return GalleryItem{}, err
	}
	if item.Nickname == "" {
		item.Nickname = s.AdminAlias()
	}
	return item, nil
}

func (s *Store) galleryCover(dateID string) (GalleryItem, error) {
	item, err := s.scanDateCover(dateID, true)
	if err == nil || err != ErrNotFound {
		return item, err
	}
	return s.scanDateCover(dateID, false)
}

func (s *Store) scanDateCover(dateID string, photosOnly bool) (GalleryItem, error) {
	query := `
SELECT m.id, m.date_id, COALESCE(m.user_id, ''), COALESCE(u.nickname, ''), m.kind, m.mime, m.name, m.created_at
FROM date_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.date_id=?`
	args := []any{dateID}
	if photosOnly {
		query += ` AND m.kind=?`
		args = append(args, GalleryKindPhoto)
	}
	query += ` ORDER BY m.created_at DESC, m.id DESC LIMIT 1`
	var item GalleryItem
	err := s.db.QueryRow(query, args...).Scan(&item.ID, &item.DateID, &item.UserID, &item.Nickname, &item.Kind, &item.MIME, &item.Name, &item.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return GalleryItem{}, ErrNotFound
		}
		return GalleryItem{}, err
	}
	if item.Nickname == "" {
		item.Nickname = s.AdminAlias()
	}
	return item, nil
}

func (s *Store) albumItemRow(album, fileID string) (GalleryItem, error) {
	if !ValidAlbum(album) {
		return GalleryItem{}, ErrNotFound
	}
	var item GalleryItem
	var userID sql.NullString
	var nickname sql.NullString
	err := s.db.QueryRow(`
SELECT m.id, m.album, m.user_id, u.nickname, m.kind, m.mime, m.name, m.created_at
FROM album_media m
LEFT JOIN users u ON u.id = m.user_id
WHERE m.id=? AND m.album=?`, fileID, album,
	).Scan(&item.ID, &item.Album, &userID, &nickname, &item.Kind, &item.MIME, &item.Name, &item.CreatedAt)
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

func (s *Store) albumPath(album, fileID string) string {
	return filepath.Join(s.mediaDir, "albums", album, fileID)
}
