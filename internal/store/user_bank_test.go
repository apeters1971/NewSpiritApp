package store

import (
	"path/filepath"
	"testing"
)

func TestUserBankDetails(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "bank.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	band, err := st.CreateUser("Cara", "cara@example.com", "secret1", RoleBand, "Drums")
	if err != nil {
		t.Fatal(err)
	}
	tech, err := st.CreateUser("Theo", "theo@example.com", "secret1", RoleTechnician, "Sound")
	if err != nil {
		t.Fatal(err)
	}
	choir, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}

	got, err := st.SetUserBank(band.ID, "DE89 3704 0044 0532 0130 00", "COBADEFFXXX")
	if err != nil {
		t.Fatal(err)
	}
	if got.IBAN != "DE89370400440532013000" || got.BIC != "COBADEFFXXX" {
		t.Fatalf("band bank %+v", got)
	}
	if _, err := st.SetUserBank(tech.ID, "DE89370400440532013000", "COBADEFF"); err != nil {
		t.Fatal(err)
	}
	cleared, err := st.SetUserBank(choir.ID, "DE89370400440532013000", "COBADEFFXXX")
	if err != nil {
		t.Fatal(err)
	}
	if cleared.IBAN != "" || cleared.BIC != "" {
		t.Fatalf("choir must not keep bank %+v", cleared)
	}
	if _, err := st.SetUserBank(band.ID, "123", ""); err == nil {
		t.Fatal("expected invalid iban")
	}
	if _, err := st.SetUserBank(band.ID, "", "NOPE"); err == nil {
		t.Fatal("expected invalid bic")
	}
	if _, err := st.UpdateUser(band.ID, "", "", "", RoleChoir, "Sopran"); err != nil {
		t.Fatal(err)
	}
	moved, err := st.UserByID(band.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.IBAN != "" || moved.BIC != "" {
		t.Fatalf("role change must clear bank %+v", moved)
	}
}
