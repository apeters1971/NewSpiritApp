package store

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"time"
)

const (
	PhotoMaxUpload = 4 << 20
	photoMaxSide   = 512
)

type Photo struct {
	MIME      string
	Data      []byte
	UpdatedAt time.Time
}

func NormalizePhoto(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("picture is required")
	}
	if len(raw) > PhotoMaxUpload {
		return nil, fmt.Errorf("picture is too large")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid picture")
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return nil, fmt.Errorf("invalid picture")
	}
	if w > photoMaxSide || h > photoMaxSide {
		if w > h {
			h = h * photoMaxSide / w
			w = photoMaxSide
		} else {
			w = w * photoMaxSide / h
			h = photoMaxSide
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		img = scaleNearest(img, w, h)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("invalid picture")
	}
	return buf.Bytes(), nil
}

func scaleNearest(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + y*sh/h
		for x := 0; x < w; x++ {
			sx := sb.Min.X + x*sw/w
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func (s *Store) SetPhoto(userID string, data []byte) error {
	if _, err := s.UserByID(userID); err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("picture is required")
	}
	_, err := s.db.Exec(`
INSERT INTO user_photos(user_id, mime, data, updated_at) VALUES(?,?,?,?)
ON CONFLICT(user_id) DO UPDATE SET mime=excluded.mime, data=excluded.data, updated_at=excluded.updated_at`,
		userID, "image/jpeg", data, fmtTime(now()),
	)
	return err
}

func (s *Store) Photo(userID string) (Photo, error) {
	var p Photo
	var updated string
	err := s.db.QueryRow(`SELECT mime, data, updated_at FROM user_photos WHERE user_id=?`, userID).Scan(&p.MIME, &p.Data, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Photo{}, ErrNotFound
	}
	if err != nil {
		return Photo{}, err
	}
	p.UpdatedAt = parseTime(updated)
	return p, nil
}

func (s *Store) DeletePhoto(userID string) error {
	if _, err := s.UserByID(userID); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM user_photos WHERE user_id=?`, userID)
	return err
}

func (s *Store) photoTimes() (map[string]time.Time, error) {
	rows, err := s.db.Query(`SELECT user_id, updated_at FROM user_photos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var id, updated string
		if err := rows.Scan(&id, &updated); err != nil {
			return nil, err
		}
		out[id] = parseTime(updated)
	}
	return out, rows.Err()
}

func (s *Store) attachPhotos(users []User) error {
	if len(users) == 0 {
		return nil
	}
	times, err := s.photoTimes()
	if err != nil {
		return err
	}
	for i := range users {
		if t, ok := times[users[i].ID]; ok {
			users[i].HasPhoto = true
			tt := t
			users[i].PhotoUpdatedAt = &tt
		}
	}
	return nil
}

func (s *Store) attachPhoto(u *User) error {
	if u == nil || u.ID == "" {
		return nil
	}
	var updated string
	err := s.db.QueryRow(`SELECT updated_at FROM user_photos WHERE user_id=?`, u.ID).Scan(&updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	u.HasPhoto = true
	t := parseTime(updated)
	u.PhotoUpdatedAt = &t
	return nil
}
