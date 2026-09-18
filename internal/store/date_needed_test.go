package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDateNeededBandVisibility(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "needed.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	cara, err := st.CreateUser("Cara", "cara@example.com", "secret1", RoleBand, "Drums")
	if err != nil {
		t.Fatal(err)
	}
	dan, err := st.CreateUser("Dan", "dan@example.com", "secret1", RoleBand, "Guitar")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	legacy, err := st.CreateDate("Whole band", CategoryRehearsal, start, nil, "", "", "", []string{RoleBand}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	picked, err := st.CreateDate("Cara only", CategoryRehearsal, start.Add(24*time.Hour), nil, "", "", "", []string{RoleChoir, RoleBand}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDateNeeded(picked.ID, []string{RoleBand}, []string{cara.ID}); err != nil {
		t.Fatal(err)
	}

	legacyViews, err := st.ListDateViews(&cara)
	if err != nil {
		t.Fatal(err)
	}
	if !dateViewHas(legacyViews, legacy.ID) || !dateViewHas(legacyViews, picked.ID) {
		t.Fatalf("cara should see both %+v", idsOf(legacyViews))
	}
	danViews, err := st.ListDateViews(&dan)
	if err != nil {
		t.Fatal(err)
	}
	if !dateViewHas(danViews, legacy.ID) || dateViewHas(danViews, picked.ID) {
		t.Fatalf("dan should see only legacy %+v", idsOf(danViews))
	}
	adaViews, err := st.ListDateViews(&ada)
	if err != nil {
		t.Fatal(err)
	}
	if dateViewHas(adaViews, legacy.ID) || !dateViewHas(adaViews, picked.ID) {
		t.Fatalf("ada should see choir+band pick %+v", idsOf(adaViews))
	}

	view, err := st.DateView(picked.ID, &cara)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Roster) != 2 {
		t.Fatalf("roster should be ada+cara, got %+v", view.Roster)
	}
	for _, row := range view.Roster {
		if row.UserID == dan.ID {
			t.Fatalf("dan on roster %+v", view.Roster)
		}
	}

	cal, err := st.ListAcceptedDatesForUser(dan)
	if err != nil {
		t.Fatal(err)
	}
	if len(cal) != 0 {
		t.Fatalf("voting dates should not be on calendar %+v", cal)
	}
	if _, err := st.SetDateStatus(legacy.ID, StatusAccepted); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetDateStatus(picked.ID, StatusAccepted); err != nil {
		t.Fatal(err)
	}
	cal, err = st.ListAcceptedDatesForUser(dan)
	if err != nil || len(cal) != 1 || cal[0].ID != legacy.ID {
		t.Fatalf("dan calendar %+v %v", cal, err)
	}
	cal, err = st.ListAcceptedDatesForUser(cara)
	if err != nil || len(cal) != 2 {
		t.Fatalf("cara calendar %d %v", len(cal), err)
	}
}

func dateViewHas(views []DateView, id string) bool {
	for _, v := range views {
		if v.ID == id {
			return true
		}
	}
	return false
}

func idsOf(views []DateView) []string {
	out := make([]string, 0, len(views))
	for _, v := range views {
		out = append(out, v.ID)
	}
	return out
}
