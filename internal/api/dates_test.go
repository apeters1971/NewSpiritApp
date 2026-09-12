package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
)

func plannerTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "dates.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := New(st, hub.New(), Options{ControllerSecret: "dev-secret"}, nil, nil)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, st
}

func memberClient(t *testing.T, ts *httptest.Server, email, password string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	res, err := c.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login %s: %d", email, res.StatusCode)
	}
	return c
}

func doJSON(t *testing.T, c *http.Client, method, url string, payload any) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func TestPlannerMemberDateAPI(t *testing.T) {
	ts, st := plannerTestServer(t)
	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", store.RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	ben, err := st.CreateUser("Ben", "ben@example.com", "secret1", store.RoleChoir, "Alt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(ada.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(ben.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetUserPlanner(ben.ID, true); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 4, 18, 0, 0, 0, time.UTC)
	ctrl, err := st.CreateDate("Controller night", store.CategoryConcert, start, nil, "", "", "", []string{store.RoleChoir}, store.Bring{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	adaC := memberClient(t, ts, "ada@example.com", "secret2")
	benC := memberClient(t, ts, "ben@example.com", "secret2")

	status, _ := doJSON(t, adaC, http.MethodPost, ts.URL+"/api/dates", map[string]any{
		"title":    "Ada night",
		"category": "concert",
		"startsAt": start.Add(24 * time.Hour).Format(time.RFC3339),
	})
	if status != http.StatusForbidden {
		t.Fatalf("non-planner create %d", status)
	}

	status, created := doJSON(t, benC, http.MethodPost, ts.URL+"/api/dates", map[string]any{
		"title":    "Ben night",
		"category": "concert",
		"startsAt": start.Add(24 * time.Hour).Format(time.RFC3339),
	})
	if status != http.StatusCreated {
		t.Fatalf("planner create %d %v", status, created)
	}
	date, _ := created["date"].(map[string]any)
	id, _ := date["id"].(string)
	if id == "" || date["mine"] != true || date["createdBy"] != ben.ID {
		t.Fatalf("created %+v", created)
	}
	creator, _ := date["creator"].(map[string]any)
	if creator["id"] != ben.ID || creator["nickname"] != "Ben" {
		t.Fatalf("creator %+v", created)
	}

	status, _ = doJSON(t, adaC, http.MethodDelete, ts.URL+"/api/dates/"+id, nil)
	if status != http.StatusForbidden {
		t.Fatalf("non-planner delete %d", status)
	}
	status, _ = doJSON(t, adaC, http.MethodPost, ts.URL+"/api/dates/"+id+"/status", map[string]string{"status": "accepted"})
	if status != http.StatusForbidden {
		t.Fatalf("non-planner accept %d", status)
	}
	status, _ = doJSON(t, benC, http.MethodDelete, ts.URL+"/api/dates/"+ctrl.ID, nil)
	if status != http.StatusForbidden {
		t.Fatalf("controller date delete %d", status)
	}

	status, poll := doJSON(t, benC, http.MethodPost, ts.URL+"/api/dates", map[string]any{
		"title":    "Weekend",
		"category": "event",
		"options": []map[string]string{
			{"startsAt": time.Date(2026, 11, 7, 18, 0, 0, 0, time.UTC).Format(time.RFC3339)},
			{"startsAt": time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC).Format(time.RFC3339)},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("poll create %d %v", status, poll)
	}
	pollDate, _ := poll["date"].(map[string]any)
	pollID, _ := pollDate["id"].(string)
	opts, _ := pollDate["options"].([]any)
	if pollID == "" || len(opts) < 2 {
		t.Fatalf("poll %+v", poll)
	}
	opt0, _ := opts[0].(map[string]any)
	optID, _ := opt0["id"].(string)
	status, _ = doJSON(t, benC, http.MethodPost, ts.URL+"/api/dates/"+pollID+"/status", map[string]string{"status": "accepted"})
	if status != http.StatusBadRequest {
		t.Fatalf("accept before freeze %d", status)
	}
	status, _ = doJSON(t, benC, http.MethodPost, ts.URL+"/api/dates/"+pollID+"/freeze", map[string]string{"optionId": optID})
	if status != http.StatusOK {
		t.Fatalf("freeze %d", status)
	}
	status, accepted := doJSON(t, benC, http.MethodPost, ts.URL+"/api/dates/"+pollID+"/status", map[string]string{"status": "accepted"})
	if status != http.StatusOK {
		t.Fatalf("accept %d %v", status, accepted)
	}
	status, _ = doJSON(t, benC, http.MethodDelete, ts.URL+"/api/dates/"+id, nil)
	if status != http.StatusOK {
		t.Fatalf("delete own %d", status)
	}
}
