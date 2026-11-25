// Package mail sends plain-text email through SES's SMTP interface, using only the standard
// library. The infra repo's mailer user and its SMTP password reach the container from SSM.
package mail

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	netmail "net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Message is one plain-text email. ReplyTo may be empty.
type Message struct {
	To, ReplyTo, Subject, Body string
}

// Sender delivers a message. Tests swap in a func that records it.
type Sender func(ctx context.Context, m Message) error

// SMTP sends through host:587. smtp.SendMail upgrades with STARTTLS, and PlainAuth refuses to
// send the password over an unencrypted connection. configSet must be the one the mailer's IAM
// policy names, or SES answers AccessDenied.
func SMTP(host, user, pass, from, configSet string) Sender {
	auth := smtp.PlainAuth("", user, pass, host)
	envelope := from // `Name <addr>` → addr for the SMTP envelope
	if a, err := netmail.ParseAddress(from); err == nil {
		envelope = a.Address
	}
	return func(ctx context.Context, m Message) error {
		msg := Build(from, configSet, m, time.Now())
		// shortcut: smtp.SendMail ignores ctx, so a hung SES connection holds the request until
		// the TCP timeout; switch to a net.Dialer + smtp.NewClient with a deadline if that bites.
		return smtp.SendMail(host+":587", auth, envelope, []string{m.To}, msg)
	}
}

// Log is the Sender for local dev, which has no SES credentials: it prints the message, so a
// verification code shows up in the API log.
func Log(_ context.Context, m Message) error {
	slog.Info("mail (not sent: no SES credentials)", "to", m.To, "reply_to", m.ReplyTo, "subject", m.Subject, "body", m.Body)
	return nil
}

// Build renders the RFC 5322 message. Header values have CR and LF removed so text a member typed
// (their name in a subject) can't add headers; the body is quoted-printable UTF-8, which keeps
// every line under SMTP's 998-character limit whatever was typed.
func Build(from, configSet string, m Message, now time.Time) []byte {
	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, oneLine(v)) }
	header("From", from)
	header("To", m.To)
	if m.ReplyTo != "" {
		header("Reply-To", m.ReplyTo)
	}
	header("Subject", mime.QEncoding.Encode("utf-8", oneLine(m.Subject)))
	header("Date", now.Format(time.RFC1123Z))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	if configSet != "" {
		header("X-SES-CONFIGURATION-SET", configSet)
	}
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	qp.Write([]byte(strings.ReplaceAll(m.Body, "\n", "\r\n")))
	qp.Close()
	return b.Bytes()
}

func oneLine(s string) string {
	return strings.NewReplacer("\r", "", "\n", " ").Replace(s)
}
