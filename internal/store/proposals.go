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

	ProposalVoteUp      = "up"
	ProposalVoteNeutral = "neutral"
	ProposalVoteDown    = "down"
)

type ProposalVoteCounts struct {
	Up      int `json:"up"`
	Neutral int `json:"neutral"`
	Down    int `json:"down"`
}

type SongProposal struct {
	ID        string             `json:"id"`
	UserID    string             `json:"userId"`
	Nickname  string             `json:"nickname"`
	Title     string             `json:"title"`
	URL       string             `json:"url,omitempty"`
	Status    string             `json:"status"`
	Comment   string             `json:"comment,omitempty"`
	MyVote    string             `json:"myVote,omitempty"`
	Votes     ProposalVoteCounts `json:"votes"`
	CreatedAt string             `json:"createdAt"`
	UpdatedAt string             `json:"updatedAt"`
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
	if err != nil {
		return err
	}
	return s.migrateProposalVotes()
}

func (s *Store) migrateProposalVotes() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS proposal_votes (
  proposal_id TEXT NOT NULL REFERENCES song_proposals(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  choice TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (proposal_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_proposal_votes_proposal ON proposal_votes(proposal_id);
`)
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
	return s.proposalByID(id, "")
}

func (s *Store) ListProposals(viewerID string) ([]SongProposal, error) {
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachProposalVotes(out, viewerID); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ProposalByID(id string) (SongProposal, error) {
	return s.proposalByID(id, "")
}

func (s *Store) proposalByID(id, viewerID string) (SongProposal, error) {
	row := s.db.QueryRow(`
SELECT p.id, p.user_id, u.nickname, p.title, p.url, p.status, p.comment, p.created_at, p.updated_at
FROM song_proposals p
JOIN users u ON u.id = p.user_id
WHERE p.id=?`, id)
	p, err := scanProposal(row)
	if err == sql.ErrNoRows {
		return SongProposal{}, ErrNotFound
	}
	if err != nil {
		return SongProposal{}, err
	}
	list := []SongProposal{p}
	if err := s.attachProposalVotes(list, viewerID); err != nil {
		return SongProposal{}, err
	}
	return list[0], nil
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

func (s *Store) SetProposalVote(userID, proposalID, choice string) (SongProposal, error) {
	if _, err := s.UserByID(userID); err != nil {
		return SongProposal{}, err
	}
	if _, err := s.ProposalByID(proposalID); err != nil {
		return SongProposal{}, err
	}
	choice = strings.TrimSpace(strings.ToLower(choice))
	if choice == "" {
		if _, err := s.db.Exec(`DELETE FROM proposal_votes WHERE proposal_id=? AND user_id=?`, proposalID, userID); err != nil {
			return SongProposal{}, err
		}
		return s.proposalByID(proposalID, userID)
	}
	switch choice {
	case ProposalVoteUp, ProposalVoteNeutral, ProposalVoteDown:
	default:
		return SongProposal{}, fmt.Errorf("invalid proposal vote")
	}
	_, err := s.db.Exec(`
INSERT INTO proposal_votes (proposal_id, user_id, choice, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(proposal_id, user_id) DO UPDATE SET choice=excluded.choice, updated_at=excluded.updated_at`,
		proposalID, userID, choice, fmtTime(now()))
	if err != nil {
		return SongProposal{}, err
	}
	return s.proposalByID(proposalID, userID)
}

func (s *Store) attachProposalVotes(list []SongProposal, viewerID string) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]string, len(list))
	index := map[string]int{}
	for i := range list {
		ids[i] = list[i].ID
		index[list[i].ID] = i
		list[i].Votes = ProposalVoteCounts{}
		list[i].MyVote = ""
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`
SELECT proposal_id, choice, COUNT(*)
FROM proposal_votes
WHERE proposal_id IN (`+placeholders+`)
GROUP BY proposal_id, choice`, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, choice string
		var n int
		if err := rows.Scan(&id, &choice, &n); err != nil {
			rows.Close()
			return err
		}
		i, ok := index[id]
		if !ok {
			continue
		}
		switch choice {
		case ProposalVoteUp:
			list[i].Votes.Up = n
		case ProposalVoteNeutral:
			list[i].Votes.Neutral = n
		case ProposalVoteDown:
			list[i].Votes.Down = n
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if strings.TrimSpace(viewerID) == "" {
		return nil
	}
	mine, err := s.db.Query(`
SELECT proposal_id, choice
FROM proposal_votes
WHERE user_id=? AND proposal_id IN (`+placeholders+`)`, append([]any{viewerID}, args...)...)
	if err != nil {
		return err
	}
	defer mine.Close()
	for mine.Next() {
		var id, choice string
		if err := mine.Scan(&id, &choice); err != nil {
			return err
		}
		if i, ok := index[id]; ok {
			list[i].MyVote = choice
		}
	}
	return mine.Err()
}

type proposalScanner interface {
	Scan(dest ...any) error
}

func scanProposal(row proposalScanner) (SongProposal, error) {
	var p SongProposal
	err := row.Scan(&p.ID, &p.UserID, &p.Nickname, &p.Title, &p.URL, &p.Status, &p.Comment, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
