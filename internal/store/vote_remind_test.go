package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestVoteReminderLinks(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "vote-remind.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	lead, err := st.CreateUser("Lea", "lea@example.com", "secret1", RoleChorleiter, "Chorleiter")
	if err != nil {
		t.Fatal(err)
	}
	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("Al", "al@example.com", "secret1", RoleEhemalige, "Ehemalige"); err != nil {
		t.Fatal(err)
	}
	d, err := st.CreateDate("Show", CategoryConcert, time.Now().UTC().Add(48*time.Hour), nil, "Hall", "", "", []string{RoleChoir}, Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CanSendVoteReminder(ada, d.ID); err == nil {
		t.Fatal("choir must not send")
	}
	if _, err := st.CanSendVoteReminder(lead, d.ID); err != nil {
		t.Fatal(err)
	}
	links, err := st.ReplaceVoteReminderLinks(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Email != "ada@example.com" || links[0].Token == "" {
		t.Fatalf("links %+v", links)
	}
	token := links[0].Token
	sess, err := st.OpenVoteReminder(token)
	if err != nil || sess.User.ID != ada.ID || sess.Remaining < VoteLinkUseWindow-time.Second {
		t.Fatalf("open %+v %v", sess, err)
	}
	if err := st.SetVote(ada.ID, d.ID, VoteYes); err != nil {
		t.Fatal(err)
	}
	again, err := st.ReplaceVoteReminderLinks(d.ID)
	if err != nil || again[0].Token == token {
		t.Fatalf("replace %+v %v", again, err)
	}
	if _, err := st.OpenVoteReminder(token); err == nil {
		t.Fatal("old token should die")
	}
	fresh := again[0].Token
	if _, err := st.OpenVoteReminder(fresh); err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	if err := st.forceVoteLinkTimes(fresh, time.Now().UTC().Add(time.Hour), past, past); err != nil {
		t.Fatal(err)
	}
	if _, err := st.VoteReminderSession(fresh); err == nil {
		t.Fatal("five-minute window should end")
	}
	links, err = st.ReplaceVoteReminderLinks(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	old := links[0].Token
	if err := st.forceVoteLinkTimes(old, time.Now().UTC().Add(-time.Hour), time.Time{}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.OpenVoteReminder(old); err == nil {
		t.Fatal("week-old link should die")
	}
}
