package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDateFeeBandAndOrchestra(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "fee.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	start := time.Date(2026, 12, 12, 20, 0, 0, 0, time.UTC)
	gig, err := st.CreateDate("Club night", CategoryConcert, start, nil, "Club", "", "", []string{RoleBand}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDateFee(gig.ID, 15000); err != nil {
		t.Fatal(err)
	}
	view, err := st.DateView(gig.ID, nil)
	if err != nil || view.FeeCents != 15000 {
		t.Fatalf("band fee %d %v", view.FeeCents, err)
	}

	choir, err := st.CreateDate("Choir only", CategoryRehearsal, start.Add(24*time.Hour), nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDateFee(choir.ID, 8000); err != nil {
		t.Fatal(err)
	}
	choirView, err := st.DateView(choir.ID, nil)
	if err != nil || choirView.FeeCents != 0 {
		t.Fatalf("choir must not keep a fee %d %v", choirView.FeeCents, err)
	}

	if err := st.SetDateFee(gig.ID, -1); err == nil {
		t.Fatal("negative fee")
	}
}

func TestDateFeeOverrides(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "fee-over.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cara, err := st.CreateUser("Cara", "cara@example.com", "secret1", RoleBand, "Drums")
	if err != nil {
		t.Fatal(err)
	}
	dan, err := st.CreateUser("Dan", "dan@example.com", "secret1", RoleBand, "Guitar")
	if err != nil {
		t.Fatal(err)
	}
	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 12, 13, 20, 0, 0, 0, time.UTC)
	gig, err := st.CreateDate("Club night", CategoryConcert, start, nil, "Club", "", "", []string{RoleBand}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDateFee(gig.ID, 15000); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDateFeeOverrides(gig.ID, map[string]int{cara.ID: 8000, dan.ID: 0}); err != nil {
		t.Fatal(err)
	}

	admin, err := st.DateView(gig.ID, nil)
	if err != nil || admin.FeeCents != 15000 || admin.FeeOverrides[cara.ID] != 8000 || admin.FeeOverrides[dan.ID] != 0 {
		t.Fatalf("admin view %+v %v", admin.FeeOverrides, err)
	}

	caraView, err := st.DateView(gig.ID, &cara)
	if err != nil || caraView.FeeCents != 8000 || caraView.FeeOverrides != nil {
		t.Fatalf("cara fee %d overrides %v %v", caraView.FeeCents, caraView.FeeOverrides, err)
	}
	danView, err := st.DateView(gig.ID, &dan)
	if err != nil || danView.FeeCents != 0 || danView.FeeOverrides != nil {
		t.Fatalf("dan fee %d overrides %v %v", danView.FeeCents, danView.FeeOverrides, err)
	}

	if err := st.SetDateFeeOverrides(gig.ID, map[string]int{ada.ID: 5000}); err != nil {
		t.Fatal(err)
	}
	cleared, err := st.DateView(gig.ID, nil)
	if err != nil || len(cleared.FeeOverrides) != 0 {
		t.Fatalf("choir override must be ignored %+v %v", cleared.FeeOverrides, err)
	}

	choir, err := st.CreateDate("Choir only", CategoryRehearsal, start.Add(24*time.Hour), nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDateFeeOverrides(choir.ID, map[string]int{cara.ID: 9000}); err != nil {
		t.Fatal(err)
	}
	choirView, err := st.DateView(choir.ID, nil)
	if err != nil || len(choirView.FeeOverrides) != 0 {
		t.Fatalf("choir date must not keep overrides %+v %v", choirView.FeeOverrides, err)
	}
}
