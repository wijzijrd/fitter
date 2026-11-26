package mail

import (
	"strings"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	long := strings.Repeat("é", 600) // 1200 bytes on one line: must be wrapped, and stay UTF-8
	got := string(Build("Kombeweeg <no-reply@staging.kombeweeg.com>", "fitter-staging", Message{
		To:      "dev@example.com",
		ReplyTo: "sam@example.com",
		Subject: "Idea from Zoë\r\nBcc: victim@example.com",
		Body:    "line one\n" + long,
	}, time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)))

	head, body, ok := strings.Cut(got, "\r\n\r\n")
	if !ok {
		t.Fatalf("no blank line between headers and body:\n%s", got)
	}
	for _, want := range []string{
		"From: Kombeweeg <no-reply@staging.kombeweeg.com>\r\n",
		"To: dev@example.com\r\n",
		"Reply-To: sam@example.com\r\n",
		"Subject: =?utf-8?q?Idea_from_Zo=C3=AB_Bcc:_victim@example.com?=\r\n",
		"Date: Sun, 22 Nov 2026 09:00:00 +0000\r\n",
		"Content-Transfer-Encoding: quoted-printable\r\n",
		"X-SES-CONFIGURATION-SET: fitter-staging\r\n",
	} {
		if !strings.Contains(head+"\r\n", want) {
			t.Errorf("headers missing %q:\n%s", want, head)
		}
	}
	if strings.Contains(head, "\nBcc:") {
		t.Errorf("subject injected a header:\n%s", head)
	}
	if !strings.HasPrefix(body, "line one\r\n=C3=A9") {
		t.Errorf("body not quoted-printable CRLF text: %q", body[:40])
	}
	for _, line := range strings.Split(body, "\r\n") {
		if len(line) > 76 {
			t.Fatalf("body line of %d chars exceeds quoted-printable's 76", len(line))
		}
	}
}

func TestBuildOmitsEmptyReplyTo(t *testing.T) {
	if got := string(Build("a@b.c", "", Message{To: "x@y.z", Subject: "s", Body: "b"}, time.Now())); strings.Contains(got, "Reply-To") || strings.Contains(got, "X-SES") {
		t.Errorf("unexpected optional header:\n%s", got)
	}
}
