package store

import (
	"database/sql"
	"sort"
	"strings"
)

type RankingEntry struct {
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Subrole  string `json:"subrole"`
	Score    int    `json:"score"`
	Yes      int    `json:"yes"`
	Maybe    int    `json:"maybe"`
	No       int    `json:"no"`
	Unknown  int    `json:"unknown"`
	Flipped  int    `json:"flipped"`
	Events   int    `json:"events"`
}

type SubroleStat struct {
	Subrole  string  `json:"subrole"`
	Members  int     `json:"members"`
	Score    int     `json:"score"`
	AvgScore float64 `json:"avgScore"`
	Yes      int     `json:"yes"`
	Maybe    int     `json:"maybe"`
	No       int     `json:"no"`
	Flipped  int     `json:"flipped"`
}

type Ranking struct {
	Year          int            `json:"year"`
	Leaders       []RankingEntry `json:"leaders,omitempty"`
	Entries       []RankingEntry `json:"entries"`
	Events        int            `json:"events"`
	Members       int            `json:"members"`
	TotalYes      int            `json:"totalYes"`
	TotalMaybe    int            `json:"totalMaybe"`
	TotalNo       int            `json:"totalNo"`
	TotalUnknown  int            `json:"totalUnknown"`
	TotalFlipped  int            `json:"totalFlipped"`
	AvgScore      float64        `json:"avgScore"`
	Participation float64        `json:"participation"`
	BySubrole     []SubroleStat  `json:"bySubrole"`
}

func votePoints(choice, initial, attendance string) int {
	if choice == VoteYes && attendance == AttendanceAbsent {
		return -2
	}
	if choice == VoteYes && attendance == AttendanceExcused {
		return 0
	}
	if choice == VoteYes {
		return 2
	}
	if choice == VoteMaybe {
		return 1
	}
	if choice == VoteNo && initial == VoteYes {
		return -1
	}
	return 0
}

func (s *Store) ChoirRanking(year int) (Ranking, error) {
	out := Ranking{Year: year, Entries: []RankingEntry{}, BySubrole: []SubroleStat{}}
	users, err := s.ListUsers()
	if err != nil {
		return out, err
	}
	dates, err := s.listDates()
	if err != nil {
		return out, err
	}
	scoring := make([]Date, 0, len(dates))
	ids := []string{}
	for _, d := range dates {
		if d.Status == StatusCancelled || d.PollOpen || !slicesContains(d.Roles, RoleChoir) {
			continue
		}
		if d.StartsAt.Year() != year {
			continue
		}
		scoring = append(scoring, d)
		ids = append(ids, d.ID)
	}
	votes, err := s.votesForDates(ids)
	if err != nil {
		return out, err
	}
	attendance, err := s.attendanceForDates(ids)
	if err != nil {
		return out, err
	}

	entries := []RankingEntry{}
	for _, u := range users {
		if u.Role != RoleChoir {
			continue
		}
		e := RankingEntry{UserID: u.ID, Nickname: u.Nickname, Subrole: u.Subrole, Events: len(scoring)}
		for _, d := range scoring {
			choice, initial := VoteUnknown, ""
			if v, ok := votes[d.ID][u.ID]; ok {
				choice = v.choice
				initial = v.initial
			}
			mark := ""
			if byUser, ok := attendance[d.ID]; ok {
				mark = byUser[u.ID]
			}
			if choice == VoteNotExpected {
				e.Events--
				continue
			}
			e.Score += votePoints(choice, initial, mark)
			switch choice {
			case VoteYes:
				e.Yes++
			case VoteMaybe:
				e.Maybe++
			case VoteNo:
				e.No++
				if initial == VoteYes {
					e.Flipped++
				}
			default:
				e.Unknown++
			}
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		if entries[i].Yes != entries[j].Yes {
			return entries[i].Yes > entries[j].Yes
		}
		return strings.ToLower(entries[i].Nickname) < strings.ToLower(entries[j].Nickname)
	})

	out.Entries = entries
	out.Events = len(scoring)
	out.Members = len(entries)
	sub := map[string]*SubroleStat{}
	for i := range Subroles[RoleChoir] {
		name := Subroles[RoleChoir][i]
		sub[name] = &SubroleStat{Subrole: name}
	}
	for _, e := range entries {
		out.TotalYes += e.Yes
		out.TotalMaybe += e.Maybe
		out.TotalNo += e.No
		out.TotalUnknown += e.Unknown
		out.TotalFlipped += e.Flipped
		if st, ok := sub[e.Subrole]; ok {
			st.Members++
			st.Score += e.Score
			st.Yes += e.Yes
			st.Maybe += e.Maybe
			st.No += e.No
			st.Flipped += e.Flipped
		}
		if e.Score > 0 && (len(out.Leaders) == 0 || e.Score > out.Leaders[0].Score) {
			out.Leaders = []RankingEntry{e}
		} else if e.Score > 0 && e.Score == out.Leaders[0].Score {
			out.Leaders = append(out.Leaders, e)
		}
	}
	if out.Members > 0 {
		sum := 0
		for _, e := range entries {
			sum += e.Score
		}
		out.AvgScore = float64(sum) / float64(out.Members)
	}
	denom := 0
	for _, e := range entries {
		denom += e.Events
	}
	if denom > 0 {
		out.Participation = float64(out.TotalYes+out.TotalMaybe+out.TotalNo) / float64(denom)
	}
	out.BySubrole = make([]SubroleStat, 0, len(Subroles[RoleChoir]))
	for _, name := range Subroles[RoleChoir] {
		st := *sub[name]
		if st.Members > 0 {
			st.AvgScore = float64(st.Score) / float64(st.Members)
		}
		out.BySubrole = append(out.BySubrole, st)
	}
	return out, nil
}

type storedVote struct {
	choice  string
	initial string
}

func (s *Store) votesForDates(ids []string) (map[string]map[string]storedVote, error) {
	out := map[string]map[string]storedVote{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT date_id, user_id, choice, initial_choice FROM votes WHERE date_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dateID, userID, choice string
		var initial sql.NullString
		if err := rows.Scan(&dateID, &userID, &choice, &initial); err != nil {
			return nil, err
		}
		if out[dateID] == nil {
			out[dateID] = map[string]storedVote{}
		}
		v := storedVote{choice: choice}
		if initial.Valid {
			v.initial = initial.String
		}
		out[dateID][userID] = v
	}
	return out, rows.Err()
}
