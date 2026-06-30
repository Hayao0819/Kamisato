package bugreport

import (
	"context"
	"fmt"

	"github.com/wneessen/go-mail"
)

// smtpReporter mails reports through an SMTP relay. Beyond a fixed To address it
// can also route a report to the package maintainer when their address is known.
type smtpReporter struct {
	cfg SMTPConfig
}

func newSMTP(cfg SMTPConfig) (Reporter, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("bugreport: smtp host is required")
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("bugreport: smtp from is required")
	}
	if cfg.To == "" && !cfg.ToMaintainer {
		return nil, fmt.Errorf("bugreport: smtp needs a to address or to_maintainer enabled")
	}
	return &smtpReporter{cfg: cfg}, nil
}

// recipients is the To list for a report: the fixed To plus the package
// maintainer, but only when maintainer routing is on and the address is known.
func recipients(cfg SMTPConfig, r Report) []string {
	var to []string
	if cfg.To != "" {
		to = append(to, cfg.To)
	}
	if cfg.ToMaintainer && r.MaintainerEmail != "" {
		to = append(to, r.MaintainerEmail)
	}
	return to
}

type smtpMessage struct {
	From    string
	To      []string
	ReplyTo string
	Subject string
	Body    string
}

func buildMessage(cfg SMTPConfig, r Report) smtpMessage {
	return smtpMessage{
		From:    cfg.From,
		To:      recipients(cfg, r),
		ReplyTo: r.Email,
		Subject: smtpSubject(r),
		Body:    issueBody(r),
	}
}

func smtpSubject(r Report) string {
	sev := r.Severity
	if sev == "" {
		sev = "bug"
	}
	pkg := r.Pkgname
	if r.Pkgver != "" {
		pkg += " " + r.Pkgver
	}
	return fmt.Sprintf("[%s] %s", sev, pkg)
}

func (s *smtpReporter) Report(ctx context.Context, r Report) (string, error) {
	m := buildMessage(s.cfg, r)
	if len(m.To) == 0 {
		return "", fmt.Errorf("bugreport: smtp has no recipient for this report")
	}
	msg := mail.NewMsg()
	if err := msg.From(m.From); err != nil {
		return "", err
	}
	if err := msg.To(m.To...); err != nil {
		return "", err
	}
	if m.ReplyTo != "" {
		if err := msg.ReplyTo(m.ReplyTo); err != nil {
			return "", err
		}
	}
	msg.Subject(m.Subject)
	msg.SetBodyString(mail.TypeTextPlain, m.Body)

	client, err := s.newClient()
	if err != nil {
		return "", err
	}
	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return "", fmt.Errorf("bugreport: smtp send: %w", err)
	}
	return "", nil
}

func (s *smtpReporter) newClient() (*mail.Client, error) {
	// Require TLS whenever credentials are sent so SMTP AUTH can never leak over
	// plaintext; without auth, allow opportunistic TLS so local relays still work.
	hasAuth := s.cfg.Username != "" || s.cfg.Password != ""
	policy := mail.TLSMandatory
	if !hasAuth {
		policy = mail.TLSOpportunistic
	}
	opts := []mail.Option{mail.WithTLSPortPolicy(policy)}
	// An explicit port overrides the policy's auto-selected one.
	if s.cfg.Port != 0 {
		opts = append(opts, mail.WithPort(s.cfg.Port))
	}
	if hasAuth {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(s.cfg.Username),
			mail.WithPassword(s.cfg.Password),
		)
	}
	return mail.NewClient(s.cfg.Host, opts...)
}
