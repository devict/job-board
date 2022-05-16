package services

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/devict/job-board/pkg/config"
)

type IEmailService interface {
	SendEmail(string, string, string) error
}

type EmailService struct {
	Conf *config.EmailConfig
}

func (svc *EmailService) SendEmail(recipient, subject, body string) error {
	msg := fmt.Sprintf(
		"From: devICT Job Board <%s>\nTo: %s\nSubject: %s\nContent-Type: text/html; charset=UTF-8\n\n%s",
		svc.Conf.FromEmail,
		recipient,
		subject,
		body,
	)

	host := strings.Split(svc.Conf.SMTPHost, ":")[0]
	auth := smtp.PlainAuth("", svc.Conf.SMTPUsername, svc.Conf.SMTPPassword, host)
	return smtp.SendMail(svc.Conf.SMTPHost, auth, svc.Conf.FromEmail, []string{recipient}, []byte(msg))
}
