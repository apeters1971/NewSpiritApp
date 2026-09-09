package store

import (
	"fmt"
	"slices"
	"strings"
)

const (
	RoleChoir       = "choir"
	RoleBand        = "band"
	RoleOrchestra   = "orchestra"
	RoleTechnician  = "technician"
	StatusVoting    = "voting"
	StatusAccepted  = "accepted"
	StatusCancelled = "cancelled"
	VoteYes         = "yes"
	VoteMaybe       = "maybe"
	VoteNo          = "no"
	VoteUnknown     = "unknown"
)

var Roles = []string{RoleChoir, RoleBand, RoleOrchestra, RoleTechnician}

var Subroles = map[string][]string{
	RoleChoir:      {"Sopran", "Alt", "Tenor/Bass"},
	RoleBand:       {"Drums", "Percussion", "Guitar", "Hammond", "E-Bass", "Trumpet", "Sax", "Trombone", "Piano"},
	RoleOrchestra:  {"Strings", "Woodbrass", "Brass", "Percussion", "Harp"},
	RoleTechnician: {"Sound", "Light", "Stage"},
}

var RoleLabels = map[string]string{
	RoleChoir:      "Choir",
	RoleBand:       "Band",
	RoleOrchestra:  "Orchestra",
	RoleTechnician: "Technician",
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
	return map[string]any{"roles": roles}
}
