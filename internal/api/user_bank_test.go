package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/apeters/newspirit/internal/store"
)

func TestTechnicianBankDetails(t *testing.T) {
	ts, st := plannerTestServer(t)
	tech, err := st.CreateUser("Theo", "theo@example.com", "secret1", store.RoleTechnician, "Sound")
	if err != nil {
		t.Fatal(err)
	}
	choir, err := st.CreateUser("Ada", "ada@example.com", "secret1", store.RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(tech.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(choir.ID, "secret2"); err != nil {
		t.Fatal(err)
	}

	techC := memberClient(t, ts, "theo@example.com", "secret2")
	status, out := doJSON(t, techC, http.MethodPatch, ts.URL+"/api/me/info", map[string]any{
		"iban": "DE89 3704 0044 0532 0130 00",
		"bic":  "COBADEFFXXX",
	})
	if status != http.StatusOK {
		t.Fatalf("tech info %d %v", status, out)
	}
	user, _ := out["user"].(map[string]any)
	if user["iban"] != "DE89370400440532013000" || user["bic"] != "COBADEFFXXX" {
		t.Fatalf("tech user %+v", user)
	}

	choirC := memberClient(t, ts, "ada@example.com", "secret2")
	status, out = doJSON(t, choirC, http.MethodPatch, ts.URL+"/api/me/info", map[string]any{
		"iban": "DE89370400440532013000",
		"bic":  "COBADEFFXXX",
	})
	if status != http.StatusOK {
		t.Fatalf("choir info %d %v", status, out)
	}
	user, _ = out["user"].(map[string]any)
	if user["iban"] != nil || user["bic"] != nil {
		t.Fatalf("choir must not keep bank %+v", user)
	}

	status, dir := doJSON(t, choirC, http.MethodGet, ts.URL+"/api/directory", nil)
	if status != http.StatusOK {
		t.Fatalf("directory %d %v", status, dir)
	}
	raw, err := json.Marshal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "DE893704") || strings.Contains(string(raw), "COBADEFF") {
		t.Fatalf("directory leaked bank %s", raw)
	}
}
