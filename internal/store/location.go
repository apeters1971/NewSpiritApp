package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Location struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address,omitempty"`
	OwnerID   string    `json:"ownerId,omitempty"`
	Owner     string    `json:"owner,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type DateVenue struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
	OwnerID string `json:"ownerId,omitempty"`
	Owner   string `json:"owner,omitempty"`
	Booking string `json:"booking,omitempty"`
}

func VenueLabel(name, address string) string {
	name = NormalizeName(name)
	address = strings.TrimSpace(address)
	if name == "" {
		return address
	}
	if address == "" || strings.EqualFold(name, address) {
		return name
	}
	return name + ", " + address
}

func OwnsVenue(user User, d Date) bool {
	return IsLocationOwner(user.Role) && user.ID != "" && d.Venue != nil && d.Venue.OwnerID == user.ID
}

func (s *Store) migrateLocations() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS locations (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  address TEXT NOT NULL DEFAULT '',
  owner_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_locations_owner ON locations(owner_id);
`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE dates ADD COLUMN location_id TEXT NOT NULL DEFAULT ''`)
	return nil
}

func (s *Store) ListLocations() ([]Location, error) {
	rows, err := s.db.Query(`
SELECT l.id, l.name, l.address, COALESCE(l.owner_id, ''), COALESCE(u.nickname, ''), l.created_at
FROM locations l
LEFT JOIN users u ON u.id = l.owner_id
ORDER BY l.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Location{}
	for rows.Next() {
		var loc Location
		var created string
		if err := rows.Scan(&loc.ID, &loc.Name, &loc.Address, &loc.OwnerID, &loc.Owner, &created); err != nil {
			return nil, err
		}
		loc.CreatedAt = parseTime(created)
		out = append(out, loc)
	}
	return out, rows.Err()
}

func (s *Store) LocationByID(id string) (Location, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Location{}, ErrNotFound
	}
	var loc Location
	var created string
	err := s.db.QueryRow(`
SELECT l.id, l.name, l.address, COALESCE(l.owner_id, ''), COALESCE(u.nickname, ''), l.created_at
FROM locations l
LEFT JOIN users u ON u.id = l.owner_id
WHERE l.id=?`, id).Scan(&loc.ID, &loc.Name, &loc.Address, &loc.OwnerID, &loc.Owner, &created)
	if err == sql.ErrNoRows {
		return Location{}, ErrNotFound
	}
	if err != nil {
		return Location{}, err
	}
	loc.CreatedAt = parseTime(created)
	return loc, nil
}

func (s *Store) CreateLocation(name, address, ownerID string) (Location, error) {
	name = NormalizeName(name)
	if name == "" {
		return Location{}, fmt.Errorf("name is required")
	}
	ownerID, err := s.normalizeLocationOwner(ownerID)
	if err != nil {
		return Location{}, err
	}
	loc := Location{ID: newID(), Name: name, Address: strings.TrimSpace(address), OwnerID: ownerID, CreatedAt: now()}
	if _, err := s.db.Exec(
		`INSERT INTO locations(id, name, address, owner_id, created_at) VALUES(?,?,?,?,?)`,
		loc.ID, loc.Name, loc.Address, nullIfEmpty(loc.OwnerID), fmtTime(loc.CreatedAt),
	); err != nil {
		return Location{}, err
	}
	return s.LocationByID(loc.ID)
}

func (s *Store) UpdateLocation(id, name, address, ownerID string) (Location, error) {
	if _, err := s.LocationByID(id); err != nil {
		return Location{}, err
	}
	name = NormalizeName(name)
	if name == "" {
		return Location{}, fmt.Errorf("name is required")
	}
	ownerID, err := s.normalizeLocationOwner(ownerID)
	if err != nil {
		return Location{}, err
	}
	if _, err := s.db.Exec(
		`UPDATE locations SET name=?, address=?, owner_id=? WHERE id=?`,
		name, strings.TrimSpace(address), nullIfEmpty(ownerID), id,
	); err != nil {
		return Location{}, err
	}
	return s.LocationByID(id)
}

func (s *Store) DeleteLocation(id string) error {
	if _, err := s.LocationByID(id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE dates SET location_id='' WHERE location_id=?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM locations WHERE id=?`, id)
	return err
}

func (s *Store) normalizeLocationOwner(ownerID string) (string, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "", nil
	}
	u, err := s.UserByID(ownerID)
	if err != nil {
		if err == ErrNotFound {
			return "", fmt.Errorf("unknown location owner")
		}
		return "", err
	}
	if !IsLocationOwner(u.Role) {
		return "", fmt.Errorf("this person is not a location owner")
	}
	return u.ID, nil
}

func (s *Store) SetDateVenue(dateID, locationID, locationText string) error {
	if _, err := s.dateRow(dateID); err != nil {
		return err
	}
	locationID = strings.TrimSpace(locationID)
	text := NormalizeName(locationText)
	if locationID == "" {
		_, err := s.db.Exec(`UPDATE dates SET location_id='', location=? WHERE id=?`, text, dateID)
		return err
	}
	loc, err := s.LocationByID(locationID)
	if err != nil {
		if err == ErrNotFound {
			return fmt.Errorf("unknown location")
		}
		return err
	}
	if text == "" {
		text = VenueLabel(loc.Name, loc.Address)
	}
	_, err = s.db.Exec(`UPDATE dates SET location_id=?, location=? WHERE id=?`, loc.ID, text, dateID)
	return err
}

func (s *Store) venuesForDates(ids []string) (map[string]*DateVenue, error) {
	out := map[string]*DateVenue{}
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
SELECT d.id, l.id, l.name, l.address, COALESCE(l.owner_id, ''), COALESCE(u.nickname, '')
FROM dates d
JOIN locations l ON l.id = d.location_id
LEFT JOIN users u ON u.id = l.owner_id
WHERE d.id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dateID string
		v := DateVenue{Booking: VoteUnknown}
		if err := rows.Scan(&dateID, &v.ID, &v.Name, &v.Address, &v.OwnerID, &v.Owner); err != nil {
			return nil, err
		}
		out[dateID] = &v
	}
	return out, rows.Err()
}

func (s *Store) appendVenueOwner(d Date, roster []RosterEntry) ([]RosterEntry, error) {
	if d.Venue == nil || d.Venue.OwnerID == "" {
		return roster, nil
	}
	for _, e := range roster {
		if e.UserID == d.Venue.OwnerID {
			return roster, nil
		}
	}
	u, err := s.UserByID(d.Venue.OwnerID)
	if err != nil {
		if err == ErrNotFound {
			return roster, nil
		}
		return nil, err
	}
	e := RosterEntry{UserID: u.ID, Nickname: u.Nickname, Email: u.Email, Role: u.Role, Subrole: u.Subrole, Choice: VoteUnknown}
	var choice, initial, setBy sql.NullString
	err = s.db.QueryRow(`SELECT choice, initial_choice, set_by FROM votes WHERE user_id=? AND date_id=?`, u.ID, d.ID).Scan(&choice, &initial, &setBy)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if choice.Valid && choice.String != "" {
		e.Choice = choice.String
	}
	if initial.Valid && initial.String != "" {
		s := initial.String
		e.InitialChoice = &s
	}
	if VoteIsProxy(e.UserID, setBy.String) {
		e.Proxy = true
	}
	return append(roster, e), nil
}

func stampVenueBooking(d *Date, roster []RosterEntry) {
	if d.Venue == nil {
		return
	}
	d.Venue.Booking = VoteUnknown
	if d.Venue.OwnerID == "" {
		return
	}
	for _, e := range roster {
		if e.UserID == d.Venue.OwnerID {
			d.Venue.Booking = e.Choice
			return
		}
	}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
