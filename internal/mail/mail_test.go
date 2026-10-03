package mail

import "testing"

func TestNoReplyFrom(t *testing.T) {
	want := "New Spirit <notifications@newspiritgospel.de>"
	if got := NoReplyFrom("", "choir.test"); got != want {
		t.Fatalf("empty from %q", got)
	}
	if got := NoReplyFrom("not-an-email", ""); got != want {
		t.Fatalf("invalid from %q", got)
	}
	if got := NoReplyFrom(DefaultFrom, ""); got != want {
		t.Fatalf("default from %q", got)
	}
	if got := NoReplyFrom("choir@newspiritgospel.de", ""); got != "New Spirit <choir@newspiritgospel.de>" {
		t.Fatalf("configured from %q", got)
	}
}

func TestMemorySend(t *testing.T) {
	m := &Memory{From: "choir@newspiritgospel.de"}
	if err := m.Send("ada@example.com", "Hi\nthere", "body"); err != nil {
		t.Fatal(err)
	}
	if len(m.Messages) != 1 || m.Messages[0].To != "ada@example.com" || m.Messages[0].Subject != "Hithere" {
		t.Fatalf("%+v", m.Messages)
	}
	if m.Messages[0].From != "New Spirit <choir@newspiritgospel.de>" {
		t.Fatalf("from %q", m.Messages[0].From)
	}
}
