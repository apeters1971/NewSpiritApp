package api

import (
	"net/http"
	"testing"

	"github.com/apeters/newspirit/internal/store"
)

func TestArchiveMemberUpdateTitle(t *testing.T) {
	ts, st := plannerTestServer(t)
	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", store.RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(ada.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	c := memberClient(t, ts, "ada@example.com", "secret2")
	status, created := doJSON(t, c, http.MethodPost, ts.URL+"/api/archive", map[string]any{
		"title": "Oh Happy Day", "composer": "Hawkins",
	})
	if status != http.StatusCreated {
		t.Fatalf("create %d %v", status, created)
	}
	item, _ := created["item"].(map[string]any)
	id, _ := item["id"].(string)
	if id == "" {
		t.Fatalf("item %+v", created)
	}
	status, updated := doJSON(t, c, http.MethodPatch, ts.URL+"/api/archive/"+id, map[string]any{
		"title": "  Amazing Grace  ", "composer": "Newton",
	})
	if status != http.StatusOK {
		t.Fatalf("patch %d %v", status, updated)
	}
	got, _ := updated["item"].(map[string]any)
	if got["title"] != "Amazing Grace" || got["composer"] != "Newton" {
		t.Fatalf("renamed %+v", got)
	}
	status, listed := doJSON(t, c, http.MethodGet, ts.URL+"/api/archive", nil)
	if status != http.StatusOK {
		t.Fatalf("list %d", status)
	}
	items, _ := listed["archive"].([]any)
	found := false
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["id"] == id {
			found = true
			if row["title"] != "Amazing Grace" {
				t.Fatalf("list title %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("missing from list")
	}
	status, bad := doJSON(t, c, http.MethodPatch, ts.URL+"/api/archive/"+id, map[string]any{"title": "   "})
	if status == http.StatusOK {
		t.Fatalf("empty title accepted %v", bad)
	}
}
