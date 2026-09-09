package store

import (
	"bytes"
	"image"
	"image/jpeg"
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

func TestChoirRanking(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "rank.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	ben, err := st.CreateUser("Ben", "ben@example.com", "secret1", RoleChoir, "Alt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("Cara", "cara@example.com", "secret1", RoleBand, "Drums"); err != nil {
		t.Fatal(err)
	}

	d1, err := st.CreateDate("One", CategoryRehearsal, time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC), nil, "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := st.CreateDate("Two", CategoryConcert, time.Date(2026, 4, 1, 18, 0, 0, 0, time.UTC), nil, "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	old, err := st.CreateDate("Old", CategoryEvent, time.Date(2025, 4, 1, 18, 0, 0, 0, time.UTC), nil, "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(ada.ID, d1.ID, VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(ben.ID, d1.ID, VoteMaybe); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(ada.ID, d2.ID, VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(ada.ID, d2.ID, VoteNo); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(ben.ID, d2.ID, VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(ada.ID, old.ID, VoteYes); err != nil {
		t.Fatal(err)
	}

	rank, err := st.ChoirRanking(2026)
	if err != nil {
		t.Fatal(err)
	}
	if rank.Events != 2 || rank.Members != 2 {
		t.Fatalf("scope %+v", rank)
	}
	if rank.Leader == nil || rank.Leader.Nickname != "Ben" || rank.Leader.Score != 3 {
		t.Fatalf("leader %+v", rank.Leader)
	}
	if rank.Entries[1].Nickname != "Ada" || rank.Entries[1].Score != 1 || rank.Entries[1].Flipped != 1 {
		t.Fatalf("ada %+v", rank.Entries[1])
	}
}

func TestUserPhoto(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "photo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	u, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if u.HasPhoto {
		t.Fatal("new user should have no photo")
	}

	img := image.NewRGBA(image.Rect(0, 0, 24, 12))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	data, err := NormalizePhoto(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPhoto(u.ID, data); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserByID(u.ID)
	if err != nil || !got.HasPhoto || got.PhotoUpdatedAt == nil {
		t.Fatalf("has photo %+v %v", got, err)
	}
	photo, err := st.Photo(u.ID)
	if err != nil || photo.MIME != "image/jpeg" || len(photo.Data) == 0 {
		t.Fatalf("photo %+v %v", photo, err)
	}
	list, err := st.ListUsers()
	if err != nil || len(list) != 1 || !list[0].HasPhoto {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := st.DeletePhoto(u.ID); err != nil {
		t.Fatal(err)
	}
	got, err = st.UserByID(u.ID)
	if err != nil || got.HasPhoto {
		t.Fatalf("deleted %+v %v", got, err)
	}
	if _, err := NormalizePhoto([]byte("not-an-image")); err == nil {
		t.Fatal("expected invalid picture")
	}
}

func TestUserInfo(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "info.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	u, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.SetUserInfo(u.ID, "Hall Street 1", "+49 30 1234", "1990-05-01")
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != "Hall Street 1" || got.Phone != "+49 30 1234" || got.Birthday != "1990-05-01" {
		t.Fatalf("info %+v", got)
	}
	if _, err := st.SetUserInfo(u.ID, "Hall", "123", "13.05.1990"); err == nil {
		t.Fatal("expected invalid birthday")
	}
}

func TestChatRoom(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	cara, err := st.CreateUser("Cara", "cara@example.com", "secret1", RoleBand, "Drums")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.AddChatMessage(ada.ID, RoleChoir, "Hello choir")
	if err != nil || msg.Text != "Hello choir" || msg.Nickname != "Ada" {
		t.Fatalf("add %+v %v", msg, err)
	}
	if _, err := st.AddChatMessage(cara.ID, RoleChoir, "nope"); err == nil {
		t.Fatal("band should not post in choir")
	}
	if _, err := st.ListChatMessages(cara.Role, RoleChoir, cara.ID); err == nil {
		t.Fatal("band should not read choir")
	}
	admin, err := st.AddAdminChatMessage(RoleChoir, "From admin")
	if err != nil || !admin.IsAdmin || admin.Nickname != "Admin" {
		t.Fatalf("admin %+v %v", admin, err)
	}
	if _, err := st.AddAdminChatMessage(RoleTechnician, "nope"); err == nil {
		t.Fatal("technician has no chat")
	}
	list, err := st.ListChatMessages(ada.Role, RoleChoir, ada.ID)
	if err != nil || len(list) != 2 || list[1].Text != "From admin" || !list[1].IsAdmin {
		t.Fatalf("list %+v %v", list, err)
	}
	ctrl, err := st.ListChatMessagesForRoom(RoleChoir)
	if err != nil || len(ctrl) != 2 {
		t.Fatalf("controller list %+v %v", ctrl, err)
	}

	reacted, err := st.ToggleChatReaction(ada.ID, RoleChoir, msg.ID, "👍", false)
	if err != nil || len(reacted.Reactions) != 1 || reacted.Reactions[0].Emoji != "👍" || !reacted.Reactions[0].Mine {
		t.Fatalf("react %+v %v", reacted, err)
	}
	again, err := st.ToggleChatReaction(ada.ID, RoleChoir, msg.ID, "👍", false)
	if err != nil || len(again.Reactions) != 0 {
		t.Fatalf("unreact %+v %v", again, err)
	}

	future := time.Now().UTC().Add(24 * time.Hour)
	d, err := st.CreateDate("Show", CategoryConcert, future, nil, "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	view, err := st.DateView(d.ID, &ada)
	if err != nil || !view.ChatOpen {
		t.Fatalf("future event chat should be open %+v %v", view, err)
	}
	room := EventChatRoom(d.ID)
	if _, err := st.AddChatMessage(ada.ID, room, "See you there"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddChatMessage(cara.ID, room, "nope"); err == nil {
		t.Fatal("band should not post in choir event chat")
	}
	if _, err := st.AddAdminChatMessage(room, "Doors at 7"); err != nil {
		t.Fatal(err)
	}

	past := time.Now().UTC().Add(-72 * time.Hour)
	old, err := st.CreateDate("Old show", CategoryConcert, past, nil, "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldView, err := st.DateView(old.ID, &ada)
	if err != nil || oldView.ChatOpen {
		t.Fatalf("old event chat should be closed %+v %v", oldView, err)
	}
	if _, err := st.AddChatMessage(ada.ID, EventChatRoom(old.ID), "too late"); err == nil {
		t.Fatal("closed event chat should reject posts")
	}
	yesterday := time.Now().UTC().Add(-20 * time.Hour)
	still, err := st.CreateDate("Last night", CategoryConcert, yesterday, nil, "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	stillView, err := st.DateView(still.ID, &ada)
	if err != nil || !stillView.ChatOpen {
		t.Fatalf("event from yesterday should stay open through +1 day %+v %v", stillView, err)
	}
}
