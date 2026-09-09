package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	ProposalPending  = "pending"
	ProposalAccepted = "accepted"
	ProposalDeclined = "declined"

	proposalTitleMax   = 200
	proposalURLMax     = 500
	proposalCommentMax = 2000
)

type SongProposal struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	Nickname  string `json:"nickname"`
	Title     string `json:"title"`
	URL       string `json:"url,omitempty"`
	Status    string `json:"status"`
	Comment   string `json:"comment,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *Store) migrateProposals() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS song_proposals (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  url TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  comment TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
)`)
	return err
}

func normalizeProposalTitle(title string) (string, error) {
	title = NormalizeName(title)
	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	if utf8.RuneCountInString(title) > proposalTitleMax {
		return "", fmt.Errorf("title is too long")
	}
	return title, nil
}

func normalizeProposalURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if utf8.RuneCountInString(raw) > proposalURLMax {
		return "", fmt.Errorf("url is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid url")
	}
	return raw, nil
}

func normalizeProposalComment(comment string) (string, error) {
	comment = strings.TrimSpace(comment)
	if utf8.RuneCountInString(comment) > proposalCommentMax {
		return "", fmt.Errorf("comment is too long")
	}
	return comment, nil
}

func normalizeProposalStatus(status string) (string, error) {
	switch strings.TrimSpace(status) {
	case ProposalPending, ProposalAccepted, ProposalDeclined:
		return strings.TrimSpace(status), nil
	default:
		return "", fmt.Errorf("invalid proposal status")
	}
}

func (s *Store) CreateProposal(userID, title, rawURL string) (SongProposal, error) {
	if _, err := s.UserByID(userID); err != nil {
		return SongProposal{}, err
	}
	title, err := normalizeProposalTitle(title)
	if err != nil {
		return SongProposal{}, err
	}
	rawURL, err = normalizeProposalURL(rawURL)
	if err != nil {
		return SongProposal{}, err
	}
	id := newID()
	ts := fmtTime(now())
	_, err = s.db.Exec(`
INSERT INTO song_proposals (id, user_id, title, url, status, comment, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, '', ?, ?)`, id, userID, title, rawURL, ProposalPending, ts, ts)
	if err != nil {
		return SongProposal{}, err
	}
	return s.ProposalByID(id)
}

func (s *Store) ListProposals() ([]SongProposal, error) {
	rows, err := s.db.Query(`
SELECT p.id, p.user_id, u.nickname, p.title, p.url, p.status, p.comment, p.created_at, p.updated_at
FROM song_proposals p
JOIN users u ON u.id = p.user_id
ORDER BY CASE p.status WHEN 'pending' THEN 0 WHEN 'accepted' THEN 1 ELSE 2 END, p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SongProposal{}
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) ProposalByID(id string) (SongProposal, error) {
	row := s.db.QueryRow(`
SELECT p.id, p.user_id, u.nickname, p.title, p.url, p.status, p.comment, p.created_at, p.updated_at
FROM song_proposals p
JOIN users u ON u.id = p.user_id
WHERE p.id=?`, id)
	p, err := scanProposal(row)
	if err == sql.ErrNoRows {
		return SongProposal{}, ErrNotFound
	}
	return p, err
}

func (s *Store) UpdateProposal(id string, status, comment *string) (SongProposal, error) {
	cur, err := s.ProposalByID(id)
	if err != nil {
		return SongProposal{}, err
	}
	nextStatus := cur.Status
	if status != nil {
		nextStatus, err = normalizeProposalStatus(*status)
		if err != nil {
			return SongProposal{}, err
		}
	}
	nextComment := cur.Comment
	if comment != nil {
		nextComment, err = normalizeProposalComment(*comment)
		if err != nil {
			return SongProposal{}, err
		}
	}
	_, err = s.db.Exec(`UPDATE song_proposals SET status=?, comment=?, updated_at=? WHERE id=?`,
		nextStatus, nextComment, fmtTime(now()), id)
	if err != nil {
		return SongProposal{}, err
	}
	return s.ProposalByID(id)
}

func (s *Store) DeleteProposal(id string) error {
	res, err := s.db.Exec(`DELETE FROM song_proposals WHERE id=?`, id)
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

type proposalScanner interface {
	Scan(dest ...any) error
}

func scanProposal(row proposalScanner) (SongProposal, error) {
	var p SongProposal
	err := row.Scan(&p.ID, &p.UserID, &p.Nickname, &p.Title, &p.URL, &p.Status, &p.Comment, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
