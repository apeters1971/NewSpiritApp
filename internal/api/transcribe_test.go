package api

import "testing"

func TestParseCloudflareTranscript(t *testing.T) {
	text, err := parseCloudflareTranscript([]byte(`{"success":true,"result":{"text":"Hallo Chor"}}`))
	if err != nil || text != "Hallo Chor" {
		t.Fatalf("object %q %v", text, err)
	}
	text, err = parseCloudflareTranscript([]byte(`{"success":true,"result":"Heute Probe"}`))
	if err != nil || text != "Heute Probe" {
		t.Fatalf("string %q %v", text, err)
	}
	if _, err := parseCloudflareTranscript([]byte(`{"success":false,"errors":[{"message":"nope"}]}`)); err == nil {
		t.Fatal("expected error")
	}
}
