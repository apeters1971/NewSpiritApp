package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/mail"
	"github.com/apeters/newspirit/internal/store"
)

func TestVoteReminderMailAndLink(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "vote-mail.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	box := &mail.Memory{From: "andi@choir.test"}
	s := New(st, hub.New(), Options{ControllerSecret: "dev-secret", Mailer: box}, nil, nil)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	lead, err := st.CreateUser("Lea", "lea@example.com", "secret1", store.RoleChorleiter, "Chorleiter")
	if err != nil {
		t.Fatal(err)
	}
	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", store.RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(lead.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(ada.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	d, err := st.CreateDate("Show", store.CategoryConcert, time.Now().UTC().Add(48*time.Hour), nil, "Hall", "", "", []string{store.RoleChoir}, store.Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	adaC := memberClient(t, ts, "ada@example.com", "secret2")
	status, out := doJSON(t, adaC, http.MethodPost, ts.URL+"/api/dates/"+d.ID+"/remind", nil)
	if status != http.StatusForbidden {
		t.Fatalf("choir remind %d %v", status, out)
	}

	leadC := memberClient(t, ts, "lea@example.com", "secret2")
	status, out = doJSON(t, leadC, http.MethodPost, ts.URL+"/api/dates/"+d.ID+"/remind", nil)
	if status != http.StatusOK {
		t.Fatalf("lead remind %d %v", status, out)
	}
	if out["sent"] != float64(1) || len(box.Messages) != 1 {
		t.Fatalf("sent %+v mail %+v", out, box.Messages)
	}
	if box.Messages[0].From != "New Spirit <notifications@newspiritgospel.de>" {
		t.Fatalf("from %q", box.Messages[0].From)
	}
	if !strings.Contains(box.Messages[0].Body, "/vote/") {
		t.Fatalf("body %q", box.Messages[0].Body)
	}
	body := box.Messages[0].Body
	i := strings.Index(body, "/vote/")
	token := strings.TrimSpace(strings.Split(body[i+6:], "\n")[0])

	c := &http.Client{}
	status, view := doJSON(t, c, http.MethodGet, ts.URL+"/api/vote/"+token, nil)
	if status != http.StatusOK {
		t.Fatalf("open %d %v", status, view)
	}
	if view["nickname"] != "Ada" || view["title"] != "Show" {
		t.Fatalf("view %+v", view)
	}
	status, view = doJSON(t, c, http.MethodPost, ts.URL+"/api/vote/"+token, map[string]string{"choice": "yes"})
	if status != http.StatusOK {
		t.Fatalf("vote %d %v", status, view)
	}
	if view["myChoice"] != "yes" {
		t.Fatalf("choice %+v", view)
	}
	got, err := st.DateView(d.ID, &ada)
	if err != nil || got.MyChoice != store.VoteYes {
		t.Fatalf("stored %+v %v", got, err)
	}

	ctrl := controllerClient(t, ts, "dev-secret")
	status, settings := doJSON(t, ctrl, http.MethodPatch, ts.URL+"/api/controller/settings", map[string]string{
		"adminAlias": "Admin",
		"newsTicker": "",
		"mailFrom":   "choir@newspiritgospel.de",
	})
	if status != http.StatusOK || settings["mailFrom"] != "choir@newspiritgospel.de" {
		t.Fatalf("settings %d %+v", status, settings)
	}
	box.Messages = nil
	status, out = doJSON(t, ctrl, http.MethodPost, ts.URL+"/api/controller/dates/"+d.ID+"/remind", nil)
	if status != http.StatusOK || out["sent"] != float64(1) {
		t.Fatalf("ctrl remind %d %+v", status, out)
	}
	if box.Messages[0].From != "New Spirit <choir@newspiritgospel.de>" {
		t.Fatalf("configured from %q", box.Messages[0].From)
	}
}
