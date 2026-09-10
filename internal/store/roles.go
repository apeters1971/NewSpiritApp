package store

import (
	"fmt"
	"slices"
	"strings"
)

const (
	RoleChoir       = "choir"
	RoleChorleiter  = "chorleiter"
	RoleBand        = "band"
	RoleOrchestra   = "orchestra"
	RoleTechnician  = "technician"
	RoleEhemalige   = "ehemalige"
	StatusVoting    = "voting"
	StatusAccepted  = "accepted"
	StatusCancelled = "cancelled"
	VoteYes         = "yes"
	VoteMaybe       = "maybe"
	VoteNo          = "no"
	VoteUnknown     = "unknown"
)

var Roles = []string{RoleChoir, RoleChorleiter, RoleBand, RoleOrchestra, RoleTechnician, RoleEhemalige}

var Subroles = map[string][]string{
	RoleChoir:       {"Sopran", "Alt", "Tenor/Bass"},
	RoleChorleiter:  {"Chorleiter"},
	RoleBand:        {"Drums", "Percussion", "Guitar", "Hammond", "E-Bass", "Trumpet", "Sax", "Trombone", "Piano"},
	RoleOrchestra:   {"Strings", "Woodbrass", "Brass", "Percussion", "Harp"},
	RoleTechnician:  {"Sound", "Light", "Stage"},
	RoleEhemalige:   {"Ehemalige"},
}

var RoleLabels = map[string]string{
	RoleChoir:       "Choir",
	RoleChorleiter:  "Choir Director",
	RoleBand:        "Band",
	RoleOrchestra:   "Orchestra",
	RoleTechnician:  "Technician",
	RoleEhemalige:   "Alumni",
}

func RoleSeesDate(role string, dateRoles []string) bool {
	if slicesContains(dateRoles, role) {
		return true
	}
	if !slicesContains(dateRoles, RoleChoir) {
		return false
	}
	return role == RoleChorleiter || role == RoleEhemalige
}

func RoleCanVote(role string) bool {
	return role != RoleEhemalige
}

func DateAudienceRoles(roles []string) []string {
	out := append([]string{}, roles...)
	if slicesContains(roles, RoleChoir) && !slicesContains(roles, RoleChorleiter) {
		out = append(out, RoleChorleiter)
	}
	return out
}

const (
	CategoryConcert     = "concert"
	CategoryRehearsal   = "rehearsal"
	CategoryMeeting     = "meeting"
	CategoryEvent       = "event"
	CategoryWeekend     = "choir-weekend"
	CategoryConcertTour = "concert-tour"
)

var Categories = []string{
	CategoryConcert,
	CategoryRehearsal,
	CategoryMeeting,
	CategoryEvent,
	CategoryWeekend,
	CategoryConcertTour,
}

var CategoryLabels = map[string]string{
	CategoryConcert:     "Concert",
	CategoryRehearsal:   "Rehearsal",
	CategoryMeeting:     "Meeting",
	CategoryEvent:       "Event",
	CategoryWeekend:     "Choir Weekend",
	CategoryConcertTour: "Concert Tour",
}

func ValidRole(role string) bool {
	return slices.Contains(Roles, role)
}

func ValidSubrole(role, subrole string) bool {
	return slices.Contains(Subroles[role], subrole)
}

func ValidStatus(status string) bool {
	return status == StatusVoting || status == StatusAccepted || status == StatusCancelled
}

func ValidChoice(choice string) bool {
	return choice == VoteYes || choice == VoteMaybe || choice == VoteNo || choice == VoteUnknown
}

func NormalizeRole(role string) (string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if !ValidRole(role) {
		return "", fmt.Errorf("invalid role")
	}
	return role, nil
}

func NormalizeRoles(roles []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		role, err := NormalizeRole(role)
		if err != nil {
			return nil, err
		}
		if seen[role] {
			continue
		}
		seen[role] = true
		out = append(out, role)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one role is required")
	}
	slices.Sort(out)
	return out, nil
}

func ValidCategory(category string) bool {
	return slices.Contains(Categories, category)
}

func NormalizeCategory(category string) (string, error) {
	category = strings.TrimSpace(strings.ToLower(category))
	if category == "" {
		return CategoryEvent, nil
	}
	if !ValidCategory(category) {
		return "", fmt.Errorf("invalid category")
	}
	return category, nil
}

const (
	DressNone    = ""
	DressWhite   = "white"
	DressBlack   = "black"
	DressCasual  = "casual"
)

var DressOptions = []string{DressWhite, DressBlack, DressCasual}

var DressLabels = map[string]string{
	DressWhite:  "White Cloth",
	DressBlack:  "Black Cloth",
	DressCasual: "Casual",
}

func NormalizeDress(dress string) (string, error) {
	dress = strings.TrimSpace(strings.ToLower(dress))
	if dress == "" || dress == "none" {
		return "", nil
	}
	if !slices.Contains(DressOptions, dress) {
		return "", fmt.Errorf("invalid dress option")
	}
	return dress, nil
}

func Catalog() map[string]any {
	roles := make([]map[string]any, 0, len(Roles))
	for _, role := range Roles {
		sub := append([]string{}, Subroles[role]...)
		roles = append(roles, map[string]any{
			"id":       role,
			"label":    RoleLabels[role],
			"subroles": sub,
		})
	}
	cats := make([]map[string]any, 0, len(Categories))
	for _, id := range Categories {
		cats = append(cats, map[string]any{"id": id, "label": CategoryLabels[id]})
	}
	dress := []map[string]any{{"id": "", "label": "None"}}
	for _, id := range DressOptions {
		dress = append(dress, map[string]any{"id": id, "label": DressLabels[id]})
	}
	return map[string]any{
		"roles":      roles,
		"categories": cats,
		"bring": map[string]any{
			"gear":  []map[string]string{{"id": "mic", "label": "Mic"}, {"id": "cable", "label": "Cable"}, {"id": "stand", "label": "Stand"}},
			"dress": dress,
		},
	}
}
