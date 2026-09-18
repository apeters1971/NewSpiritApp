package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLocationBookingVisibility(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "venue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	choir, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := st.CreateUser("Klaus", "klaus@example.com", "secret1", RoleLocation, "Location Owner")
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateUser("Inge", "inge@example.com", "secret1", RoleLocation, "Location Owner")
	if err != nil {
		t.Fatal(err)
	}
	hall, err := st.CreateLocation("Gemeindehaus", "Kirchweg 1", owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 12, 1, 18, 0, 0, 0, time.UTC)
	booked, err := st.CreateDate("Probe im Haus", CategoryRehearsal, start, nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDateVenue(booked.ID, hall.ID, ""); err != nil {
		t.Fatal(err)
	}
	free, err := st.CreateDate("Freie Adresse", CategoryRehearsal, start.Add(24*time.Hour), nil, "Andere Halle", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	ownerViews, err := st.ListDateViews(&owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerViews) != 1 || ownerViews[0].ID != booked.ID {
		t.Fatalf("owner should see only booked date %+v", ownerViews)
	}
	if ownerViews[0].Venue == nil || ownerViews[0].Venue.OwnerID != owner.ID {
		t.Fatalf("venue %+v", ownerViews[0].Venue)
	}
	if ownerViews[0].MyChoice != VoteUnknown {
		t.Fatalf("unvoted booking %q", ownerViews[0].MyChoice)
	}
	otherViews, err := st.ListDateViews(&other)
	if err != nil || len(otherViews) != 0 {
		t.Fatalf("other owner %d %v", len(otherViews), err)
	}
	choirViews, err := st.ListDateViews(&choir)
	if err != nil {
		t.Fatal(err)
	}
	if len(choirViews) != 2 {
		t.Fatalf("choir should see both %d", len(choirViews))
	}

	if err := st.SetVote(owner.ID, booked.ID, VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(other.ID, booked.ID, VoteYes); err == nil {
		t.Fatal("other owner must not vote")
	}
	if err := st.SetVote(owner.ID, free.ID, VoteYes); err == nil {
		t.Fatal("owner must not vote on free-text date")
	}
	view, err := st.DateView(booked.ID, &owner)
	if err != nil || view.MyChoice != VoteYes {
		t.Fatalf("booking vote %v %v", view.MyChoice, err)
	}
	if view.Venue == nil || view.Venue.Booking != VoteYes {
		t.Fatalf("venue booking %v", view.Venue)
	}
	onRoster := false
	for _, row := range view.Roster {
		if row.UserID == owner.ID {
			onRoster = true
			if row.Choice != VoteYes {
				t.Fatalf("owner roster %q", row.Choice)
			}
		}
	}
	if !onRoster {
		t.Fatal("owner missing from roster")
	}
	if _, err := st.AddComment(owner.ID, booked.ID, "Piano is already on stage"); err != nil {
		t.Fatalf("owner comment: %v", err)
	}
}

func TestLocationOwnerVotesDuringPoll(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "venue-poll.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	owner, err := st.CreateUser("Klaus", "klaus@example.com", "secret1", RoleLocation, "Location Owner")
	if err != nil {
		t.Fatal(err)
	}
	hall, err := st.CreateLocation("Saal", "", owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	a := time.Date(2026, 12, 4, 18, 0, 0, 0, time.UTC)
	b := a.Add(24 * time.Hour)
	d, err := st.CreateDate("Umfrage", CategoryEvent, time.Time{}, nil, "", "", "", []string{RoleChoir}, Bring{}, []PollOptionInput{
		{StartsAt: a},
		{StartsAt: b},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDateVenue(d.ID, hall.ID, ""); err != nil {
		t.Fatal(err)
	}
	if !d.PollOpen {
		row, err := st.dateRow(d.ID)
		if err != nil {
			t.Fatal(err)
		}
		d = row
	}
	if err := st.SetVote(owner.ID, d.ID, VoteMaybe); err != nil {
		t.Fatal(err)
	}
	view, err := st.DateView(d.ID, &owner)
	if err != nil || view.MyChoice != VoteMaybe {
		t.Fatalf("poll booking %q %v", view.MyChoice, err)
	}
}

func TestLocationLeadChat(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "lead-chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	lead, err := st.CreateUser("Andi", "andi@example.com", "secret1", RoleChorleiter, "Chorleiter")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := st.CreateUser("Klaus", "klaus@example.com", "secret1", RoleLocation, "Location Owner")
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateUser("Inge", "inge@example.com", "secret1", RoleLocation, "Location Owner")
	if err != nil {
		t.Fatal(err)
	}
	choir, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}

	chats, err := st.LeadChatsFor(owner)
	if err != nil || len(chats) != 1 || chats[0].Peer.ID != lead.ID {
		t.Fatalf("owner leads %+v %v", chats, err)
	}
	room := chats[0].Room
	if !IsLeadRoom(room) || LeadRoom(owner.ID, lead.ID) != room {
		t.Fatalf("room %q", room)
	}
	leadChats, err := st.LeadChatsFor(lead)
	if err != nil || len(leadChats) != 2 {
		t.Fatalf("director leads %d %v", len(leadChats), err)
	}

	if _, err := st.AddChatMessage(owner.ID, room, "Keys are at the back"); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListChatMessages(owner.Role, room, owner.ID)
	if err != nil || len(list) != 1 || list[0].Text != "Keys are at the back" {
		t.Fatalf("owner read %+v %v", list, err)
	}
	if _, err := st.ListChatMessages(lead.Role, room, lead.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListChatMessages(other.Role, room, other.ID); err == nil {
		t.Fatal("other owner must not read this lead chat")
	}
	if _, err := st.AddChatMessage(choir.ID, room, "nope"); err == nil {
		t.Fatal("choir must not post in lead chat")
	}
	if _, err := st.AddChatMessage(owner.ID, RoleChoir, "nope"); err == nil {
		t.Fatal("owner must not post in choir chat")
	}
}
