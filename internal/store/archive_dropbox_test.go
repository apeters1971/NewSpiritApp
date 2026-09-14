package store

import (
	"path/filepath"
	"testing"
)

func TestArchiveDropboxImport(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "dropbox.db"))
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

	drop, err := st.AddDropboxFile("Oh_Happy_Day.txt", ada.ID, []byte("Oh Happy Day lyrics"))
	if err != nil || drop.Kind != ArchiveKindLyrics || drop.Name != "Oh_Happy_Day.txt" || drop.CreatedByName != "Ada" {
		t.Fatalf("add %+v %v", drop, err)
	}
	if _, err := st.AddDropboxFile("notes.doc", ada.ID, []byte("nope")); err == nil {
		t.Fatal("bad type")
	}

	own, err := st.ListDropbox(false, ada.ID)
	if err != nil || len(own) != 1 {
		t.Fatalf("own %+v %v", own, err)
	}
	others, err := st.ListDropbox(false, ben.ID)
	if err != nil || len(others) != 0 {
		t.Fatalf("ben %d %v", len(others), err)
	}
	all, err := st.ListDropbox(true, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("all %d %v", len(all), err)
	}

	if !st.CanManageDropbox(ada, drop) || st.CanManageDropbox(ben, drop) {
		t.Fatal("manage")
	}
	ada, err = st.SetUserArchiver(ada.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !st.CanManageDropbox(ada, drop) {
		t.Fatal("archiver")
	}

	updated, err := st.UpdateDropboxMeta(drop.ID, "Oh Happy Day", "Edwin Hawkins", "choir")
	if err != nil || updated.Title != "Oh Happy Day" || updated.Author != "Edwin Hawkins" || updated.Instrument != "choir" {
		t.Fatalf("meta %+v %v", updated, err)
	}

	item, err := st.ImportDropbox(drop.ID, "", "", "")
	if err != nil || item.Title != "Oh Happy Day" || item.Composer != "Edwin Hawkins" || item.Status != ArchiveStatusAccepted {
		t.Fatalf("import %+v %v", item, err)
	}
	if len(item.Files) != 1 || item.Files[0].Kind != ArchiveKindLyrics || item.Files[0].Role != "choir" || item.Files[0].Name != "Oh_Happy_Day.txt" {
		t.Fatalf("file %+v", item.Files)
	}
	left, err := st.ListDropbox(true, "")
	if err != nil || len(left) != 0 {
		t.Fatalf("left %d %v", len(left), err)
	}
}

func TestArchiveDropboxAttach(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "dropbox-attach.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	song, err := st.CreateArchiveItem("Oh Happy Day", "Edwin Hawkins")
	if err != nil {
		t.Fatal(err)
	}
	drop, err := st.AddDropboxFile("choir.pdf", ada.ID, []byte("%PDF-1.4 choir"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateDropboxMeta(drop.ID, "", "", "choir"); err != nil {
		t.Fatal(err)
	}
	item, err := st.AttachDropbox(drop.ID, song.ID, "")
	if err != nil || item.ID != song.ID || item.Title != "Oh Happy Day" {
		t.Fatalf("attach %+v %v", item, err)
	}
	if len(item.Files) != 1 || item.Files[0].Kind != ArchiveKindSheet || item.Files[0].Role != "choir" || item.Files[0].Name != "choir.pdf" {
		t.Fatalf("file %+v", item.Files)
	}
	left, err := st.ListDropbox(true, "")
	if err != nil || len(left) != 0 {
		t.Fatalf("left %d %v", len(left), err)
	}
}
