package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestVoteAndAccept(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	choir, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("Ben", "ben@example.com", "secret1", RoleChoir, "Alt"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("Cara", "cara@example.com", "secret1", RoleBand, "Drums"); err != nil {
		t.Fatal(err)
	}

	start := time.Date(2026, 9, 20, 18, 0, 0, 0, time.UTC)
	d, err := st.CreateDate("Sunday practice", CategoryRehearsal, start, nil, "Hall", "", []string{RoleChoir}, Bring{Mic: true, Dress: DressBlack}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Category != CategoryRehearsal {
		t.Fatalf("category %s", d.Category)
	}
	if !d.Bring.Mic || d.Bring.Dress != DressBlack {
		t.Fatalf("bring %+v", d.Bring)
	}
	if err := st.SetVote(choir.ID, d.ID, VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(choir.ID, d.ID, VoteMaybe); err != nil {
		t.Fatal(err)
	}

	view, err := st.DateView(d.ID, &choir)
	if err != nil {
		t.Fatal(err)
	}
	if view.MyChoice != VoteMaybe || view.MyInitial == nil || *view.MyInitial != VoteYes {
		t.Fatalf("first vote should stay Yes, got %+v", view)
	}
	if len(view.Roster) != 2 {
		t.Fatalf("roster %d", len(view.Roster))
	}
	if len(view.SubroleCounts) != 2 {
		t.Fatalf("counts %d", len(view.SubroleCounts))
	}

	if _, err := st.SetDateStatus(d.ID, StatusAccepted); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(choir.ID, d.ID, VoteNo); err != nil {
		t.Fatal(err)
	}
	view, err = st.DateView(d.ID, &choir)
	if err != nil {
		t.Fatal(err)
	}
	if view.MyChoice != VoteNo || view.MyInitial == nil || *view.MyInitial != VoteYes {
		t.Fatalf("first vote should still be Yes, got %+v", view)
	}

	if _, err := st.AddComment(choir.ID, d.ID, "I can come after rehearsal"); err != nil {
		t.Fatal(err)
	}
	view, err = st.DateView(d.ID, &choir)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Comments) != 1 || view.Comments[0].Text != "I can come after rehearsal" {
		t.Fatalf("comments %+v", view.Comments)
	}
}

func TestPollFreeze(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "poll.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	sat := time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)
	sun := time.Date(2026, 10, 4, 16, 0, 0, 0, time.UTC)
	d, err := st.CreateDate("Weekend show", CategoryConcert, time.Time{}, nil, "Hall", "", []string{RoleChoir}, Bring{}, []PollOptionInput{
		{StartsAt: sat},
		{StartsAt: sun},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !d.PollOpen || len(d.Options) != 2 {
		t.Fatalf("poll %+v", d)
	}
	if err := st.SetVote(ada.ID, d.ID, VoteYes); err == nil {
		t.Fatal("date vote should be blocked while poll is open")
	}
	if err := st.SetPollVote(ada.ID, d.ID, d.Options[1].ID, VoteYes); err != nil {
		t.Fatal(err)
	}
	pre, err := st.DateView(d.ID, &ada)
	if err != nil {
		t.Fatal(err)
	}
	if pre.Options[1].MyChoice != VoteYes || pre.Options[1].Yes != 1 {
		t.Fatalf("poll vote not visible %+v", pre.Options[1])
	}
	if _, err := st.SetDateStatus(d.ID, StatusAccepted); err == nil {
		t.Fatal("accept should wait for freeze")
	}
	frozen, err := st.FreezePoll(d.ID, d.Options[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.PollOpen || frozen.FrozenOptionID != d.Options[1].ID || !frozen.StartsAt.Equal(sun) {
		t.Fatalf("frozen %+v", frozen)
	}
	view, err := st.DateView(d.ID, &ada)
	if err != nil {
		t.Fatal(err)
	}
	if view.MyChoice != VoteYes {
		t.Fatalf("seeded vote %s", view.MyChoice)
	}
	if err := st.SetPollVote(ada.ID, d.ID, d.Options[0].ID, VoteMaybe); err == nil {
		t.Fatal("poll vote should lock after freeze")
	}
}
