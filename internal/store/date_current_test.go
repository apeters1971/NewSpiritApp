package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPastDateHidesMemberVote(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "past.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	start := time.Now().Add(-48 * time.Hour).UTC()
	past, err := st.CreateDate("Old night", CategoryConcert, start, nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if DateIsCurrent(mustDate(t, st, past.ID)) {
		t.Fatal("yesterday should not be current")
	}

	soon := time.Now().Add(48 * time.Hour).UTC()
	next, err := st.CreateDate("Soon", CategoryConcert, soon, nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !DateIsCurrent(mustDate(t, st, next.ID)) {
		t.Fatal("future date should be current")
	}
}

func mustDate(t *testing.T, st *Store, id string) Date {
	t.Helper()
	d, err := st.dateRow(id)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
