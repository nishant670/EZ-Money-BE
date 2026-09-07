package mailer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"strings"
	"time"
)

// buildMIME renders msg as RFC 5322 bytes suitable for SMTP DATA. A message
// with an HTML body becomes multipart/alternative with the plain text first,
// which is the ordering that makes a text-only client show the text part.
//
// Both bodies are quoted-printable encoded so that a long line, a rupee sign,
// or a leading "." cannot corrupt the transfer.
func buildMIME(from Address, msg Message, now time.Time) ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", now.Format(time.RFC1123Z))
	messageID, err := generateMessageID(from.Email)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&b, "Message-ID: %s\r\n", messageID)
	b.WriteString("MIME-Version: 1.0\r\n")
	// A one-time code is worthless once used and should not sit in a search
	// index or a spam-filter cache any longer than it is valid.
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("X-Auto-Response-Suppress: All\r\n")

	if strings.TrimSpace(msg.HTML) == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		encoded, err := encodeQuotedPrintable(msg.Text)
		if err != nil {
			return nil, err
		}
		b.WriteString(encoded)
		return []byte(b.String()), nil
	}

	boundary, err := generateBoundary()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)

	for _, part := range []struct{ contentType, body string }{
		{"text/plain; charset=UTF-8", msg.Text},
		{"text/html; charset=UTF-8", msg.HTML},
	} {
		encoded, err := encodeQuotedPrintable(part.body)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		fmt.Fprintf(&b, "Content-Type: %s\r\n", part.contentType)
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		b.WriteString(encoded)
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String()), nil
}

func encodeQuotedPrintable(body string) (string, error) {
	var out strings.Builder
	writer := quotedprintable.NewWriter(&out)
	// Normalise to CRLF first: a bare LF reaching the wire splits the message
	// for some servers and silently truncates it for others.
	normalised := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n")
	if _, err := writer.Write([]byte(normalised)); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func generateBoundary() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "finnri-" + hex.EncodeToString(raw), nil
}

func generateMessageID(fromEmail string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	domain := "finnri.local"
	if at := strings.LastIndex(fromEmail, "@"); at >= 0 && at+1 < len(fromEmail) {
		domain = fromEmail[at+1:]
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(raw), domain), nil
}
