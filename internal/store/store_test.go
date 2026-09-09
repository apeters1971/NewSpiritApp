package store

import (
	"bytes"
	"image"
	"image/jpeg"
	"path/filepath"
	"strings"
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
	d, err := st.CreateDate("Sunday practice", CategoryRehearsal, start, nil, "Hall", "", "17:30 Soundcheck\n18:00 Beginn", []string{RoleChoir}, Bring{Mic: true, Dress: DressBlack}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Category != CategoryRehearsal {
		t.Fatalf("category %s", d.Category)
	}
	if d.Schedule != "17:30 Soundcheck\n18:00 Beginn" {
		t.Fatalf("schedule %q", d.Schedule)
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
	d, err := st.CreateDate("Weekend show", CategoryConcert, time.Time{}, nil, "Hall", "", "", []string{RoleChoir}, Bring{}, []PollOptionInput{
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

	d1, err := st.CreateDate("One", CategoryRehearsal, time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC), nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := st.CreateDate("Two", CategoryConcert, time.Date(2026, 4, 1, 18, 0, 0, 0, time.UTC), nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	old, err := st.CreateDate("Old", CategoryEvent, time.Date(2025, 4, 1, 18, 0, 0, 0, time.UTC), nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
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
	if len(rank.Leaders) != 1 || rank.Leaders[0].Nickname != "Ben" || rank.Leaders[0].Score != 3 {
		t.Fatalf("leader %+v", rank.Leaders)
	}
	if rank.Entries[1].Nickname != "Ada" || rank.Entries[1].Score != 1 || rank.Entries[1].Flipped != 1 {
		t.Fatalf("ada %+v", rank.Entries[1])
	}

	dana, err := st.CreateUser("Dana", "dana@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(dana.ID, d1.ID, VoteMaybe); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVote(dana.ID, d2.ID, VoteYes); err != nil {
		t.Fatal(err)
	}
	tied, err := st.ChoirRanking(2026)
	if err != nil || len(tied.Leaders) != 2 || tied.Leaders[0].Score != 3 || tied.Leaders[1].Score != 3 {
		t.Fatalf("tie %+v %v", tied.Leaders, err)
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

func TestDirectory(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "directory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("Ben", "ben@example.com", "secret1", RoleChoir, "Alt"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetUserInfo(ada.ID, "Hidden Street", "+49 611 1234", "1990-05-01"); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d people", len(list))
	}
	if list[0].Nickname != "Ada" || list[0].Email != "ada@example.com" || list[0].Phone != "+49 611 1234" {
		t.Fatalf("ada %+v", list[0])
	}
	if list[1].Nickname != "Ben" || list[1].Phone != "" {
		t.Fatalf("ben %+v", list[1])
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
	d, err := st.CreateDate("Show", CategoryConcert, future, nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
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
	old, err := st.CreateDate("Old show", CategoryConcert, past, nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
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
	still, err := st.CreateDate("Last night", CategoryConcert, yesterday, nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	stillView, err := st.DateView(still.ID, &ada)
	if err != nil || !stillView.ChatOpen {
		t.Fatalf("event from yesterday should stay open through +1 day %+v %v", stillView, err)
	}

	if got := st.AdminAlias(); got != "Admin" {
		t.Fatalf("default alias %q", got)
	}
	alias, err := st.SetAdminAlias("  Leitung  ")
	if err != nil || alias != "Leitung" {
		t.Fatalf("set alias %q %v", alias, err)
	}
	named, err := st.AddAdminChatMessage(RoleChoir, "From Leitung")
	if err != nil || named.Nickname != "Leitung" {
		t.Fatalf("named admin %+v %v", named, err)
	}
	namedList, err := st.ListChatMessages(ada.Role, RoleChoir, ada.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range namedList {
		if m.IsAdmin && m.Nickname != "Leitung" {
			t.Fatalf("listed admin nickname %q", m.Nickname)
		}
		if m.ID == named.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("missing aliased admin message")
	}
	if _, err := st.SetAdminAlias(strings.Repeat("x", 41)); err == nil {
		t.Fatal("expected long alias error")
	}

	if err := st.DeleteChatMessage(cara.ID, RoleChoir, msg.ID, false); err == nil {
		t.Fatal("band should not delete choir message")
	}
	if err := st.DeleteChatMessage(ada.ID, RoleChoir, admin.ID, false); err == nil {
		t.Fatal("member should not delete admin message")
	}
	if err := st.DeleteChatMessage(ada.ID, RoleChoir, msg.ID, false); err != nil {
		t.Fatalf("delete own %v", err)
	}
	after, err := st.ListChatMessages(ada.Role, RoleChoir, ada.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range after {
		if m.ID == msg.ID {
			t.Fatal("deleted message still listed")
		}
	}
	if err := st.DeleteChatMessage("", RoleChoir, named.ID, true); err != nil {
		t.Fatalf("admin delete %v", err)
	}
}

func TestArchiveAndDateTitles(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateArchiveItem("", ""); err == nil {
		t.Fatal("empty title")
	}
	song, err := st.CreateArchiveItem("Amazing Grace", "Traditional")
	if err != nil || song.Title != "Amazing Grace" || song.Composer != "Traditional" {
		t.Fatalf("create %+v %v", song, err)
	}
	other, err := st.CreateArchiveItem("Oh Happy Day", "")
	if err != nil {
		t.Fatal(err)
	}
	found, err := st.ListArchive("grace")
	if err != nil || len(found) != 1 || found[0].ID != song.ID {
		t.Fatalf("search %+v %v", found, err)
	}
	lyrics := []byte("Amazing grace, how sweet the sound")
	if _, err := st.AddArchiveFile(song.ID, ArchiveKindLyrics, "choir", "lyrics.txt", lyrics); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddArchiveFile(song.ID, ArchiveKindSheet, "guitar", "choir.pdf", []byte("%PDF-1.4 choir")); err != nil {
		t.Fatal(err)
	}
	mp3 := []byte{0xFF, 0xFB, 0x90, 0x00, 'I', 'D', '3'}
	item, err := st.AddArchiveFile(song.ID, ArchiveKindAudio, "piano", "demo.mp3", mp3)
	if err != nil || len(item.Files) != 3 {
		t.Fatalf("files %+v %v", item, err)
	}
	again, err := st.AddArchiveFile(song.ID, ArchiveKindAudio, "choir", "choir.mp3", mp3)
	if err != nil || len(again.Files) != 4 {
		t.Fatalf("append audio %+v %v", again, err)
	}
	var pianoID string
	for _, f := range again.Files {
		if f.Kind == ArchiveKindAudio && f.Role == "piano" {
			pianoID = f.ID
		}
	}
	renamed, err := st.UpdateArchiveFile(song.ID, pianoID, "Piano track", "piano")
	if err != nil {
		t.Fatal(err)
	}
	foundName := false
	for _, f := range renamed.Files {
		if f.ID == pianoID && f.Name == "Piano track" {
			foundName = true
		}
	}
	if !foundName {
		t.Fatalf("rename %+v", renamed.Files)
	}

	d, err := st.CreateDate("Show", CategoryConcert, time.Now().UTC().Add(24*time.Hour), nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MemberCanAccessArchive(ada.ID, song.ID); err == nil {
		t.Fatal("should not access before titles are set")
	}
	if err := st.SetDateTitles(d.ID, []string{song.ID, other.ID}); err != nil {
		t.Fatal(err)
	}
	view, err := st.DateView(d.ID, &ada)
	if err != nil || len(view.Titles) != 2 || view.Titles[0].ID != song.ID {
		t.Fatalf("titles %+v %v", view.Titles, err)
	}
	if err := st.MemberCanAccessArchive(ada.ID, song.ID); err != nil {
		t.Fatalf("access %v", err)
	}
	if err := st.SetDateTitles(d.ID, []string{other.ID}); err != nil {
		t.Fatal(err)
	}
	view, err = st.DateView(d.ID, &ada)
	if err != nil || len(view.Titles) != 1 || view.Titles[0].ID != other.ID {
		t.Fatalf("replaced titles %+v %v", view.Titles, err)
	}
	var lyricsID string
	for _, f := range renamed.Files {
		if f.Kind == ArchiveKindLyrics {
			lyricsID = f.ID
		}
	}
	if _, err := st.DeleteArchiveFile(song.ID, lyricsID); err != nil {
		t.Fatal(err)
	}
	gone, err := st.ArchiveItem(song.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range gone.Files {
		if f.Kind == ArchiveKindLyrics {
			t.Fatal("lyrics should be gone")
		}
	}
	if err := st.DeleteArchiveItem(song.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ArchiveItem(song.ID); err == nil {
		t.Fatal("deleted archive item still there")
	}
}

func TestChannels(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "channels.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	tech, err := st.CreateUser("Tim", "tim@example.com", "secret1", RoleTechnician, "Sound")
	if err != nil {
		t.Fatal(err)
	}
	all, err := st.ListChannels()
	if err != nil || len(all) != ChannelMax {
		t.Fatalf("list %d %v", len(all), err)
	}
	ch, err := st.SetChannel(12, ada.ID, "  Headset  ", true)
	if err != nil || ch.Number != 12 || ch.UserID != ada.ID || ch.Nickname != "Ada" || ch.Comment != "Headset" || !ch.V48 {
		t.Fatalf("set %+v %v", ch, err)
	}
	if _, err := st.SetChannel(13, tech.ID, "", false); err == nil {
		t.Fatal("technician should not get a channel")
	}
	if _, err := st.SetChannel(0, ada.ID, "", false); err == nil {
		t.Fatal("channel 0")
	}
	got, err := st.UserByID(ada.ID)
	if err != nil || len(got.Channels) != 1 || got.Channels[0].Number != 12 {
		t.Fatalf("user channels %+v %v", got.Channels, err)
	}
	named, err := st.SetChannelComment(ada.ID, 12, "  In-ear  ", false)
	if err != nil || named.Comment != "In-ear" || named.V48 {
		t.Fatalf("own comment %+v %v", named, err)
	}
	if _, err := st.SetChannelComment(tech.ID, 12, "nope", true); err == nil {
		t.Fatal("other user should not edit comment")
	}
	if _, err := st.SetChannel(12, "", "spare", true); err != nil {
		t.Fatal(err)
	}
	got, err = st.UserByID(ada.ID)
	if err != nil || len(got.Channels) != 0 {
		t.Fatalf("cleared %+v %v", got.Channels, err)
	}
}

func TestSongProposals(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "prop.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProposal(ada.ID, "  ", ""); err == nil {
		t.Fatal("empty title should fail")
	}
	if _, err := st.CreateProposal(ada.ID, "Oceans", "javascript:alert(1)"); err == nil {
		t.Fatal("bad url should fail")
	}
	p, err := st.CreateProposal(ada.ID, "  Oceans  ", "https://example.com/oceans")
	if err != nil || p.Title != "Oceans" || p.Status != ProposalPending || p.URL != "https://example.com/oceans" {
		t.Fatalf("create %+v %v", p, err)
	}
	accepted := ProposalAccepted
	note := "  Sunday set  "
	p, err = st.UpdateProposal(p.ID, &accepted, &note)
	if err != nil || p.Status != ProposalAccepted || p.Comment != "Sunday set" {
		t.Fatalf("update %+v %v", p, err)
	}
	list, err := st.ListProposals()
	if err != nil || len(list) != 1 || list[0].Nickname != "Ada" {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := st.DeleteProposal(p.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteProposal(p.ID); err != ErrNotFound {
		t.Fatalf("second delete %v", err)
	}
}

func TestCalendarFeed(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "cal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 20, 18, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	accepted, err := st.CreateDate("Sunday practice", CategoryRehearsal, start, &end, "Hall", "Bring scores", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetDateStatus(accepted.ID, StatusAccepted); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDate("Band only", CategoryRehearsal, start.Add(24*time.Hour), nil, "", "", "", []string{RoleBand}, Bring{}, nil); err != nil {
		t.Fatal(err)
	}
	voting, err := st.CreateDate("Open vote", CategoryConcert, start.Add(48*time.Hour), nil, "", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if voting.Status != StatusVoting {
		t.Fatalf("status %s", voting.Status)
	}

	token, err := st.EnsureCalendarToken(ada.ID)
	if err != nil || token == "" {
		t.Fatalf("token %q %v", token, err)
	}
	again, err := st.EnsureCalendarToken(ada.ID)
	if err != nil || again != token {
		t.Fatalf("token should stay %q got %q %v", token, again, err)
	}
	user, err := st.UserByCalendarToken(token)
	if err != nil || user.ID != ada.ID || user.Role != RoleChoir {
		t.Fatalf("by token %+v %v", user, err)
	}
	if _, err := st.UserByCalendarToken("nope"); err != ErrNotFound {
		t.Fatalf("bad token %v", err)
	}
	dates, err := st.ListAcceptedDatesForRole(RoleChoir)
	if err != nil || len(dates) != 1 || dates[0].ID != accepted.ID {
		t.Fatalf("accepted %+v %v", dates, err)
	}
	ics := RenderICS("New Spirit", dates)
	if !strings.Contains(ics, "BEGIN:VCALENDAR") || !strings.Contains(ics, "SUMMARY:Sunday practice") {
		t.Fatalf("ics %s", ics)
	}
	if !strings.Contains(ics, "LOCATION:Hall") || strings.Contains(ics, "Open vote") || strings.Contains(ics, "Band only") {
		t.Fatalf("ics filter %s", ics)
	}
}

func TestPasswordMustChange(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "pw.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil || !ada.MustChangePassword {
		t.Fatalf("create %+v %v", ada, err)
	}
	if _, err := st.ChangeOwnPassword(ada.ID, "secret1"); err == nil {
		t.Fatal("same password should be rejected")
	}
	changed, err := st.ChangeOwnPassword(ada.ID, "secret2")
	if err != nil || changed.MustChangePassword {
		t.Fatalf("change %+v %v", changed, err)
	}
	if _, _, err := st.Login("ada@example.com", "secret2"); err != nil {
		t.Fatal(err)
	}
	reset, err := st.UpdateUser(ada.ID, "", "", "secret3", "", "")
	if err != nil || !reset.MustChangePassword {
		t.Fatalf("reset %+v %v", reset, err)
	}
}
