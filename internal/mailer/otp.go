package mailer

import (
	"fmt"
	"html/template"
	"strings"
)

// OTPEmail renders the sign-in code email. The code appears in the subject as
// well as the body because most phone mail clients show enough of the subject
// on the notification for the person to read it without opening anything, and
// both iOS and Android autofill a code from a subject line of this shape.
func OTPEmail(to, code string, expiresInMinutes int) Message {
	minutes := "a few minutes"
	if expiresInMinutes > 0 {
		minutes = fmt.Sprintf("%d minutes", expiresInMinutes)
	}
	return Message{
		To:      to,
		Subject: fmt.Sprintf("%s is your Finnri code", code),
		Text:    otpText(code, minutes),
		HTML:    otpHTML(code, minutes),
	}
}

func otpText(code, minutes string) string {
	var b strings.Builder
	b.WriteString("Your Finnri verification code is:\n\n")
	b.WriteString(code)
	b.WriteString("\n\nIt expires in " + minutes + " and can be used once.\n\n")
	b.WriteString("If you did not ask to sign in to Finnri, you can ignore this email — ")
	b.WriteString("nobody can get in with this code alone.\n\n")
	b.WriteString("— Finnri\n")
	return b.String()
}

// otpHTMLTemplate is deliberately table-free, inline-styled and single-column.
// Anything cleverer renders unpredictably across Gmail, Outlook and Apple Mail,
// and this email has exactly one job.
var otpHTMLTemplate = template.Must(template.New("otp").Parse(`<!doctype html>
<html>
<body style="margin:0;padding:24px;background:#f5f5f4;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#2d2d2d;">
  <div style="max-width:420px;margin:0 auto;background:#ffffff;border-radius:16px;padding:32px;">
    <p style="margin:0 0 8px;font-size:15px;line-height:1.5;">Your Finnri verification code is</p>
    <p style="margin:0 0 20px;font-size:34px;font-weight:700;letter-spacing:6px;font-variant-numeric:tabular-nums;">{{.Code}}</p>
    <p style="margin:0 0 20px;font-size:14px;line-height:1.6;color:#57534e;">It expires in {{.Minutes}} and can be used once.</p>
    <p style="margin:0;font-size:13px;line-height:1.6;color:#78716c;">If you did not ask to sign in to Finnri, you can ignore this email — nobody can get in with this code alone.</p>
  </div>
  <p style="max-width:420px;margin:16px auto 0;font-size:12px;color:#a8a29e;text-align:center;">Finnri</p>
</body>
</html>`))

func otpHTML(code, minutes string) string {
	var out strings.Builder
	// The template cannot fail on these inputs, but swallowing the error would
	// silently ship an empty HTML part; falling back to text-only is honest.
	if err := otpHTMLTemplate.Execute(&out, struct{ Code, Minutes string }{code, minutes}); err != nil {
		return ""
	}
	return out.String()
}
