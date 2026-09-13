package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type PollOptionInput struct {
	ID       string
	StartsAt time.Time
	EndsAt   *time.Time
}

type PollOption struct {
	ID        string         `json:"id"`
	StartsAt  time.Time      `json:"startsAt"`
	EndsAt    *time.Time     `json:"endsAt,omitempty"`
	Frozen    bool           `json:"frozen"`
	MyChoice  string         `json:"myChoice"`
	MyInitial *string        `json:"myInitial,omitempty"`
	MyProxy   bool           `json:"myProxy,omitempty"`
	Yes       int            `json:"yes"`
	Maybe     int            `json:"maybe"`
	No        int            `json:"no"`
	Unknown   int            `json:"unknown"`
	Roster    []RosterEntry  `json:"roster"`
}

type pollVoteRow struct {
	UserID        string
	Choice        string
	InitialChoice string
	SetBy         string
}

func resolveSchedule(starts time.Time, ends *time.Time, options []PollOptionInput) (time.Time, *time.Time, []PollOptionInput, error) {
	cleaned := make([]PollOptionInput, 0, len(options))
	for _, o := range options {
		if o.StartsAt.IsZero() {
			continue
		}
		if o.EndsAt != nil && o.EndsAt.Before(o.StartsAt) {
			return time.Time{}, nil, nil, fmt.Errorf("end time is before start time")
		}
		o.StartsAt = o.StartsAt.UTC()
		o.EndsAt = utcPtr(o.EndsAt)
		cleaned = append(cleaned, o)
	}
	if len(cleaned) == 1 {
		return cleaned[0].StartsAt, cleaned[0].EndsAt, nil, nil
	}
	if len(cleaned) >= 2 {
		earliest := cleaned[0]
		for _, o := range cleaned[1:] {
			if o.StartsAt.Before(earliest.StartsAt) {
				earliest = o
			}
		}
		return earliest.StartsAt, earliest.EndsAt, cleaned, nil
	}
	if starts.IsZero() {
		return time.Time{}, nil, nil, fmt.Errorf("start time is required")
	}
	if ends != nil && ends.Before(starts) {
		return time.Time{}, nil, nil, fmt.Errorf("end time is before start time")
	}
	return starts.UTC(), utcPtr(ends), nil, nil
}

func pollOpen(frozenID string, options []PollOption) bool {
	return frozenID == "" && len(options) >= 2
}

func (s *Store) replacePollOptions(tx *sql.Tx, dateID string, inputs []PollOptionInput) error {
	keep := map[string]bool{}
	for i, in := range inputs {
		id := strings.TrimSpace(in.ID)
		var ends any
		if in.EndsAt != nil {
			ends = fmtTime(*in.EndsAt)
		}
		if id != "" {
			var exists int
			if err := tx.QueryRow(`SELECT COUNT(1) FROM poll_options WHERE id=? AND date_id=?`, id, dateID).Scan(&exists); err != nil {
				return err
			}
			if exists == 1 {
				if _, err := tx.Exec(
					`UPDATE poll_options SET starts_at=?, ends_at=?, sort_order=? WHERE id=?`,
					fmtTime(in.StartsAt), ends, i, id,
				); err != nil {
					return err
				}
				keep[id] = true
				continue
			}
		}
		id = newID()
		if _, err := tx.Exec(
			`INSERT INTO poll_options(id, date_id, starts_at, ends_at, sort_order) VALUES(?,?,?,?,?)`,
			id, dateID, fmtTime(in.StartsAt), ends, i,
		); err != nil {
			return err
		}
		keep[id] = true
	}
	rows, err := tx.Query(`SELECT id FROM poll_options WHERE date_id=?`, dateID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var drop []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if !keep[id] {
			drop = append(drop, id)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range drop {
		if _, err := tx.Exec(`DELETE FROM poll_options WHERE id=?`, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) optionsForDates(ids []string) (map[string][]PollOption, error) {
	out := map[string][]PollOption{}
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
SELECT id, date_id, starts_at, ends_at
FROM poll_options
WHERE date_id IN (`+placeholders+`)
ORDER BY sort_order, starts_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var opt PollOption
		var dateID, starts string
		var ends sql.NullString
		if err := rows.Scan(&opt.ID, &dateID, &starts, &ends); err != nil {
			return nil, err
		}
		opt.StartsAt = parseTime(starts)
		opt.EndsAt = parseTimePtr(ends)
		opt.MyChoice = VoteUnknown
		opt.Roster = []RosterEntry{}
		out[dateID] = append(out[dateID], opt)
	}
	return out, rows.Err()
}

func (s *Store) pollVotesForOptions(optionIDs []string) (map[string]map[string]pollVoteRow, error) {
	out := map[string]map[string]pollVoteRow{}
	if len(optionIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(optionIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(optionIDs))
	for i, id := range optionIDs {
		args[i] = id
	}
	rows, err := s.db.Query(`
SELECT option_id, user_id, choice, initial_choice, set_by
FROM poll_votes
WHERE option_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var optionID string
		var initial, setBy sql.NullString
		var row pollVoteRow
		if err := rows.Scan(&optionID, &row.UserID, &row.Choice, &initial, &setBy); err != nil {
			return nil, err
		}
		if initial.Valid {
			row.InitialChoice = initial.String
		}
		if setBy.Valid {
			row.SetBy = setBy.String
		}
		if out[optionID] == nil {
			out[optionID] = map[string]pollVoteRow{}
		}
		out[optionID][row.UserID] = row
	}
	return out, rows.Err()
}

func attachPoll(options []PollOption, frozenID string, roster []RosterEntry, votes map[string]map[string]pollVoteRow, viewer *User) []PollOption {
	if options == nil {
		return []PollOption{}
	}
	for i := range options {
		opt := &options[i]
		opt.Frozen = frozenID != "" && opt.ID == frozenID
		opt.MyChoice = VoteUnknown
		opt.Roster = make([]RosterEntry, 0, len(roster))
		optVotes := votes[opt.ID]
		for _, person := range roster {
			entry := person
			entry.Choice = VoteUnknown
			entry.InitialChoice = nil
			entry.Proxy = false
			if v, ok := optVotes[person.UserID]; ok {
				entry.Choice = v.Choice
				if v.InitialChoice != "" {
					init := v.InitialChoice
					entry.InitialChoice = &init
				}
				entry.Proxy = VoteIsProxy(person.UserID, v.SetBy)
			}
			switch entry.Choice {
			case VoteYes:
				opt.Yes++
			case VoteMaybe:
				opt.Maybe++
			case VoteNo:
				opt.No++
			case VoteNotExpected:
			default:
				opt.Unknown++
			}
			opt.Roster = append(opt.Roster, entry)
			if viewer != nil && person.UserID == viewer.ID {
				opt.MyChoice = entry.Choice
				opt.MyInitial = entry.InitialChoice
				opt.MyProxy = entry.Proxy
			}
		}
	}
	return options
}

func (s *Store) SetPollVote(userID, dateID, optionID, choice string) error {
	return s.setPollVote(userID, userID, dateID, optionID, choice)
}

func (s *Store) SetPollVoteFor(actorID, userID, dateID, optionID, choice string) error {
	if actorID == "" || actorID == userID {
		return s.setPollVote(userID, userID, dateID, optionID, choice)
	}
	actor, err := s.UserByID(actorID)
	if err != nil {
		return err
	}
	d, err := s.dateRow(dateID)
	if err != nil {
		return err
	}
	if err := plannerMaySetVote(actor, d); err != nil {
		return err
	}
	return s.setPollVote(actor.ID, userID, dateID, optionID, choice)
}

func (s *Store) SetAdminPollVote(userID, dateID, optionID, choice string) error {
	return s.setPollVote(VoteSetByAdmin, userID, dateID, optionID, choice)
}

func (s *Store) setPollVote(setBy, userID, dateID, optionID, choice string) error {
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
	if !d.PollOpen {
		return fmt.Errorf("%w: poll is frozen", ErrForbidden)
	}
	found := false
	for _, o := range d.Options {
		if o.ID == optionID {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	u, err := s.UserByID(userID)
	if err != nil {
		return err
	}
	if !RoleCanVote(u.Role) || !slicesContains(d.Roles, u.Role) {
		return fmt.Errorf("%w: this date is not for your role", ErrForbidden)
	}
	if setBy == "" {
		setBy = userID
	}
	_, err = s.db.Exec(`
INSERT INTO poll_votes(user_id, option_id, choice, initial_choice, set_by, updated_at) VALUES(?,?,?,?,?,?)
ON CONFLICT(user_id, option_id) DO UPDATE SET choice=excluded.choice, set_by=excluded.set_by, updated_at=excluded.updated_at`,
		userID, optionID, choice, choice, setBy, fmtTime(now()),
	)
	return err
}

func (s *Store) FreezePoll(dateID, optionID string) (Date, error) {
	d, err := s.dateRow(dateID)
	if err != nil {
		return Date{}, err
	}
	if d.Status != StatusVoting {
		return Date{}, fmt.Errorf("date is already %s", d.Status)
	}
	if !d.PollOpen {
		return Date{}, fmt.Errorf("poll is frozen")
	}
	var chosen *PollOption
	for i := range d.Options {
		if d.Options[i].ID == optionID {
			chosen = &d.Options[i]
			break
		}
	}
	if chosen == nil {
		return Date{}, ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Date{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var ends any
	if chosen.EndsAt != nil {
		ends = fmtTime(*chosen.EndsAt)
	}
	if _, err := tx.Exec(
		`UPDATE dates SET frozen_option_id=?, starts_at=?, ends_at=? WHERE id=?`,
		chosen.ID, fmtTime(chosen.StartsAt), ends, dateID,
	); err != nil {
		return Date{}, err
	}
	rows, err := tx.Query(`SELECT user_id, choice, initial_choice, set_by, updated_at FROM poll_votes WHERE option_id=?`, chosen.ID)
	if err != nil {
		return Date{}, err
	}
	type seed struct {
		userID, choice, initial, setBy, updated string
	}
	var seeds []seed
	for rows.Next() {
		var item seed
		var initial, setBy sql.NullString
		if err := rows.Scan(&item.userID, &item.choice, &initial, &setBy, &item.updated); err != nil {
			rows.Close()
			return Date{}, err
		}
		if initial.Valid {
			item.initial = initial.String
		} else {
			item.initial = item.choice
		}
		if setBy.Valid {
			item.setBy = setBy.String
		} else {
			item.setBy = item.userID
		}
		seeds = append(seeds, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Date{}, err
	}
	for _, item := range seeds {
		if _, err := tx.Exec(`
INSERT INTO votes(user_id, date_id, choice, initial_choice, set_by, updated_at) VALUES(?,?,?,?,?,?)
ON CONFLICT(user_id, date_id) DO UPDATE SET choice=excluded.choice, set_by=excluded.set_by, updated_at=excluded.updated_at`,
			item.userID, dateID, item.choice, item.initial, item.setBy, item.updated,
		); err != nil {
			return Date{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Date{}, err
	}
	return s.dateRow(dateID)
}
