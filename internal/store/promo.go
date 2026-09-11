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
	PromoKindTicket = "ticket"
	PromoKindFlyer  = "flyer"
	PromoKindPoster = "poster"
	PromoMaxBytes   = 12 << 20
	promoNoteMax    = 2000
)

type PromoItem struct {
	ID        string `json:"id"`
	DateID    string `json:"dateId"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	URL       string `json:"url,omitempty"`
	MIME      string `json:"mime,omitempty"`
	CreatedAt string `json:"createdAt"`
}

func ValidPromoKind(kind string) bool {
	return kind == PromoKindTicket || kind == PromoKindFlyer || kind == PromoKindPoster
}

func ValidPromoFileKind(kind string) bool {
	return kind == PromoKindFlyer || kind == PromoKindPoster
}

func DateAllowsPromo(category string) bool {
	return category == CategoryConcert
}

func (s *Store) requirePromoDate(dateID string) error {
	d, err := s.dateRow(dateID)
	if err != nil {
		return err
	}
	if !DateAllowsPromo(d.Category) {
		return fmt.Errorf("promo is only for concerts")
	}
	return nil
}

func (s *Store) migratePromo() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS date_promo (
  id TEXT PRIMARY KEY,
  date_id TEXT NOT NULL REFERENCES dates(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  url TEXT NOT NULL DEFAULT '',
  mime TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_date_promo_date ON date_promo(date_id, kind, created_at);
CREATE TABLE IF NOT EXISTS date_promo_notes (
  date_id TEXT PRIMARY KEY REFERENCES dates(id) ON DELETE CASCADE,
  note TEXT NOT NULL DEFAULT ''
);
`)
	return err
}

func (s *Store) PromoNote(dateID string) (string, error) {
	if err := s.requirePromoDate(dateID); err != nil {
		return "", err
	}
	var note string
	err := s.db.QueryRow(`SELECT note FROM date_promo_notes WHERE date_id=?`, dateID).Scan(&note)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return note, nil
}

func (s *Store) SetPromoNote(dateID, note string) (string, error) {
	if err := s.requirePromoDate(dateID); err != nil {
		return "", err
	}
	note, err := preparePromoNote(note)
	if err != nil {
		return "", err
	}
	if note == "" {
		_, err = s.db.Exec(`DELETE FROM date_promo_notes WHERE date_id=?`, dateID)
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO date_promo_notes(date_id, note) VALUES(?,?)
		 ON CONFLICT(date_id) DO UPDATE SET note=excluded.note`,
		dateID, note,
	)
	if err != nil {
		return "", err
	}
	return note, nil
}

func preparePromoNote(note string) (string, error) {
	note = strings.TrimSpace(strings.ReplaceAll(note, "\r\n", "\n"))
	if utf8.RuneCountInString(note) > promoNoteMax {
		return "", fmt.Errorf("comment is too long")
	}
	return note, nil
}

func (s *Store) promoCounts(ids []string) (map[string]int, error) {
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
	rows, err := s.db.Query(`SELECT date_id, COUNT(*) FROM date_promo WHERE date_id IN (`+placeholders+`) GROUP BY date_id`, args...)
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

func (s *Store) ListPromo(dateID string) ([]PromoItem, error) {
	if err := s.requirePromoDate(dateID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, date_id, kind, name, url, mime, created_at FROM date_promo WHERE date_id=? ORDER BY kind, created_at, name COLLATE NOCASE`,
		dateID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PromoItem{}
	for rows.Next() {
		item, err := scanPromo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) AddPromoTicket(dateID, name, rawURL string) (PromoItem, error) {
	if err := s.requirePromoDate(dateID); err != nil {
		return PromoItem{}, err
	}
	rawURL, err := prepareArchiveURL(rawURL)
	if err != nil {
		return PromoItem{}, err
	}
	name, err = prepareArchiveLabel(name, linkLabelFromURL(rawURL))
	if err != nil {
		return PromoItem{}, err
	}
	item := PromoItem{
		ID:        newID(),
		DateID:    dateID,
		Kind:      PromoKindTicket,
		Name:      name,
		URL:       rawURL,
		CreatedAt: fmtTime(now()),
	}
	_, err = s.db.Exec(
		`INSERT INTO date_promo(id, date_id, kind, name, url, mime, created_at) VALUES(?,?,?,?,?,?,?)`,
		item.ID, item.DateID, item.Kind, item.Name, item.URL, "", item.CreatedAt,
	)
	if err != nil {
		return PromoItem{}, err
	}
	return item, nil
}

func (s *Store) AddPromoFile(dateID, kind, filename string, r io.Reader) (PromoItem, error) {
	if !ValidPromoFileKind(kind) {
		return PromoItem{}, fmt.Errorf("unknown promo kind")
	}
	if err := s.requirePromoDate(dateID); err != nil {
		return PromoItem{}, err
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return PromoItem{}, err
	}
	head = head[:n]
	if len(head) == 0 {
		return PromoItem{}, fmt.Errorf("file is required")
	}
	mime, err := sniffPromoFile(filename, head)
	if err != nil {
		return PromoItem{}, err
	}
	name, err := prepareArchiveLabel(sanitizeArchiveName(filename), "file")
	if err != nil {
		return PromoItem{}, err
	}
	id := newID()
	dest := s.promoPath(dateID, id)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return PromoItem{}, err
	}
	f, err := os.Create(dest)
	if err != nil {
		return PromoItem{}, err
	}
	written, copyErr := io.Copy(f, io.LimitReader(io.MultiReader(bytes.NewReader(head), r), PromoMaxBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return PromoItem{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return PromoItem{}, closeErr
	}
	if written > PromoMaxBytes {
		_ = os.Remove(dest)
		return PromoItem{}, fmt.Errorf("file is too large")
	}
	item := PromoItem{
		ID:        id,
		DateID:    dateID,
		Kind:      kind,
		Name:      name,
		MIME:      mime,
		CreatedAt: fmtTime(now()),
	}
	_, err = s.db.Exec(
		`INSERT INTO date_promo(id, date_id, kind, name, url, mime, created_at) VALUES(?,?,?,?,?,?,?)`,
		item.ID, item.DateID, item.Kind, item.Name, "", item.MIME, item.CreatedAt,
	)
	if err != nil {
		_ = os.Remove(dest)
		return PromoItem{}, err
	}
	return item, nil
}

func (s *Store) PromoFilePath(dateID, fileID string) (PromoItem, string, error) {
	if err := s.requirePromoDate(dateID); err != nil {
		return PromoItem{}, "", err
	}
	item, err := s.promoItemRow(dateID, fileID)
	if err != nil {
		return PromoItem{}, "", err
	}
	if item.Kind == PromoKindTicket {
		return PromoItem{}, "", ErrNotFound
	}
	p := s.promoPath(dateID, fileID)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return PromoItem{}, "", ErrNotFound
		}
		return PromoItem{}, "", err
	}
	return item, p, nil
}

func (s *Store) DeletePromoItem(dateID, fileID string) error {
	if err := s.requirePromoDate(dateID); err != nil {
		return err
	}
	item, err := s.promoItemRow(dateID, fileID)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM date_promo WHERE id=? AND date_id=?`, fileID, dateID); err != nil {
		return err
	}
	if item.Kind != PromoKindTicket {
		_ = os.Remove(s.promoPath(dateID, fileID))
	}
	return nil
}

func (s *Store) promoItemRow(dateID, fileID string) (PromoItem, error) {
	row := s.db.QueryRow(
		`SELECT id, date_id, kind, name, url, mime, created_at FROM date_promo WHERE id=? AND date_id=?`,
		fileID, dateID,
	)
	item, err := scanPromo(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return PromoItem{}, ErrNotFound
		}
		return PromoItem{}, err
	}
	return item, nil
}

func scanPromo(rs rowScanner) (PromoItem, error) {
	var item PromoItem
	if err := rs.Scan(&item.ID, &item.DateID, &item.Kind, &item.Name, &item.URL, &item.MIME, &item.CreatedAt); err != nil {
		return PromoItem{}, err
	}
	return item, nil
}

func (s *Store) promoPath(dateID, fileID string) string {
	return filepath.Join(filepath.Dir(s.mediaDir), "promo", dateID, fileID)
}

func (s *Store) removePromoDir(dateID string) error {
	return os.RemoveAll(filepath.Join(filepath.Dir(s.mediaDir), "promo", dateID))
}

func sniffPromoFile(filename string, data []byte) (string, error) {
	ext := strings.ToLower(path.Ext(filename))
	detected := http.DetectContentType(data)
	if i := strings.IndexByte(detected, ';'); i >= 0 {
		detected = strings.TrimSpace(detected[:i])
	}
	switch {
	case len(data) >= 4 && string(data[:4]) == "%PDF" || detected == "application/pdf":
		return "application/pdf", nil
	case detected == "image/jpeg" || ext == ".jpg" || ext == ".jpeg":
		return "image/jpeg", nil
	case ext == ".pdf":
		return "application/pdf", nil
	default:
		return "", fmt.Errorf("file type is not allowed")
	}
}
