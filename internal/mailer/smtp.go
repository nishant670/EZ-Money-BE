package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPOptions configures the SMTP driver. It is the universal driver: Resend,
// Amazon SES, Brevo, Postmark, Mailgun and Gmail all speak SMTP, so choosing a
// provider is an env-var change rather than a code change.
type SMTPOptions struct {
	Host     string
	Port     int
	Username string
	Password string
	// TLSMode is "starttls" (port 587, the usual choice), "tls" (implicit TLS
	// on port 465), or "none". "none" is only for a local capture server such
	// as MailHog and refuses to carry a password.
	TLSMode string
	From    Address
	// Dialer is overridable so tests can point at a local listener.
	Dialer *net.Dialer
}

type smtpSender struct {
	opts SMTPOptions
}

// NewSMTPSender builds the SMTP driver. Configuration errors are reported on
// the first Send rather than at construction, so a misconfigured mail server
// cannot stop the API process from booting and serving everything else.
func NewSMTPSender(opts SMTPOptions) Sender {
	if opts.Port == 0 {
		opts.Port = 587
	}
	if strings.TrimSpace(opts.TLSMode) == "" {
		opts.TLSMode = "starttls"
	}
	if opts.Dialer == nil {
		opts.Dialer = &net.Dialer{Timeout: 10 * time.Second}
	}
	return &smtpSender{opts: opts}
}

func (s *smtpSender) Name() string { return "smtp" }

func (s *smtpSender) validate() error {
	if strings.TrimSpace(s.opts.Host) == "" {
		return errors.New("mailer: SMTP_HOST is required when EMAIL_PROVIDER=smtp")
	}
	if err := s.opts.From.valid(); err != nil {
		return err
	}
	switch strings.ToLower(s.opts.TLSMode) {
	case "starttls", "tls", "none":
	default:
		return fmt.Errorf("mailer: unknown SMTP_TLS_MODE %q (want starttls, tls or none)", s.opts.TLSMode)
	}
	if strings.EqualFold(s.opts.TLSMode, "none") && s.opts.Password != "" {
		return errors.New("mailer: refusing to send an SMTP password over an unencrypted connection")
	}
	return nil
}

func (s *smtpSender) Send(ctx context.Context, msg Message) error {
	if err := s.validate(); err != nil {
		return err
	}
	if err := msg.Validate(); err != nil {
		return err
	}
	body, err := buildMIME(s.opts.From, msg, time.Now())
	if err != nil {
		return fmt.Errorf("mailer: build message: %w", err)
	}

	addr := net.JoinHostPort(s.opts.Host, strconv.Itoa(s.opts.Port))
	conn, err := s.dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("mailer: dial %s: %w", addr, err)
	}
	defer conn.Close()

	// The context deadline has to reach the socket too: without this a hung
	// server holds the HTTP handler open for the full request timeout.
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, s.opts.Host)
	if err != nil {
		return fmt.Errorf("mailer: smtp handshake: %w", err)
	}
	defer client.Close()

	if strings.EqualFold(s.opts.TLSMode, "starttls") {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return fmt.Errorf("mailer: %s does not offer STARTTLS", addr)
		}
		if err := client.StartTLS(&tls.Config{ServerName: s.opts.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("mailer: starttls: %w", err)
		}
	}

	if s.opts.Username != "" {
		auth := smtp.PlainAuth("", s.opts.Username, s.opts.Password, s.opts.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mailer: smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.opts.From.Email); err != nil {
		return fmt.Errorf("mailer: MAIL FROM: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("mailer: RCPT TO: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA: %w", err)
	}
	if _, err := writer.Write(body); err != nil {
		return fmt.Errorf("mailer: write body: %w", err)
	}
	// The server's acceptance is reported by Close, not by Write. Returning
	// before this is what "otp_sent" used to mean, and it meant nothing.
	if err := writer.Close(); err != nil {
		return fmt.Errorf("mailer: server rejected message: %w", err)
	}
	return client.Quit()
}

func (s *smtpSender) dial(ctx context.Context, addr string) (net.Conn, error) {
	if strings.EqualFold(s.opts.TLSMode, "tls") {
		return (&tls.Dialer{
			NetDialer: s.opts.Dialer,
			Config:    &tls.Config{ServerName: s.opts.Host, MinVersion: tls.VersionTLS12},
		}).DialContext(ctx, "tcp", addr)
	}
	return s.opts.Dialer.DialContext(ctx, "tcp", addr)
}
