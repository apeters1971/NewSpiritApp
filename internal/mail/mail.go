package mail

import (
	"bytes"
	"fmt"
	"net/mail"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

const (
	fromName    = "New Spirit"
	DefaultFrom = "notifications@newspiritgospel.de"
)

type Sender interface {
	Send(to, subject, body string) error
}

type Record struct {
	To      string
	Subject string
	Body    string
	From    string
}

type Memory struct {
	From     string
	Domain   string
	Messages []Record
}

func (m *Memory) Send(to, subject, body string) error {
	to, err := sanitizeAddr(to)
	if err != nil {
		return err
	}
	from := NoReplyFrom(m.From, m.Domain)
	m.Messages = append(m.Messages, Record{To: to, Subject: oneLine(subject), Body: body, From: from})
	return nil
}

type Sendmail struct {
	Path   string
	From   string
	Domain string
}

func (s Sendmail) Send(to, subject, body string) error {
	to, err := sanitizeAddr(to)
	if err != nil {
		return err
	}
	from := NoReplyFrom(s.From, s.Domain)
	path := s.Path
	if path == "" {
		path = sendmailPath()
	}
	msg := formatMessage(from, to, oneLine(subject), body)
	cmd := exec.Command(path, "-t", "-i")
	cmd.Stdin = bytes.NewReader(msg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if text == "" {
			return fmt.Errorf("sendmail: %w", err)
		}
		return fmt.Errorf("sendmail: %s", text)
	}
	return nil
}

func sendmailPath() string {
	if p := strings.TrimSpace(os.Getenv("SENDMAIL_PATH")); p != "" {
		return p
	}
	for _, p := range []string{"/usr/sbin/sendmail", "/usr/bin/sendmail"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("sendmail"); err == nil {
		return p
	}
	return "/usr/sbin/sendmail"
}

func NoReplyFrom(from, domain string) string {
	email := extractAddr(strings.TrimSpace(from))
	if _, err := sanitizeAddr(email); err != nil {
		email = DefaultFrom
	}
	return fromName + " <" + email + ">"
}

func extractAddr(raw string) string {
	if raw == "" {
		return ""
	}
	if addr, err := mail.ParseAddress(raw); err == nil && addr.Address != "" {
		return addr.Address
	}
	return raw
}

func sanitizeAddr(raw string) (string, error) {
	raw = extractAddr(oneLine(raw))
	if raw == "" || !strings.Contains(raw, "@") {
		return "", fmt.Errorf("invalid email")
	}
	if _, err := mail.ParseAddress(raw); err != nil {
		return "", fmt.Errorf("invalid email")
	}
	return raw, nil
}

func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}

func formatMessage(from, to, subject, body string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&b, "Auto-Submitted: auto-generated\r\n")
	fmt.Fprintf(&b, "\r\n")
	b.WriteString(strings.ReplaceAll(body, "\r\n", "\n"))
	if !strings.HasSuffix(body, "\n") {
		b.WriteByte('\n')
	}
	return b.Bytes()
}
