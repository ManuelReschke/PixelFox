package mail

import (
	"fmt"
	"log"
	"net/smtp"

	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
)

// SMTPMailer sends emails via SMTP
func SendMail(to string, subject string, body string) error {
	host := env.GetEnv("SMTP_HOST", "")
	port := env.GetEnv("SMTP_PORT", "")
	username := env.GetEnv("SMTP_USERNAME", "")
	password := env.GetEnv("SMTP_PASSWORD", "")
	sender := env.GetEnv("SMTP_SENDER", "")

	if sender == "" {
		sender = fmt.Sprintf("no-reply@%s", "localhost")
		log.Printf("SMTP_SENDER not set, using default sender: %s", sender)
	}

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	msg := []byte(
		fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n", sender, to, subject) +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
			body,
	)

	err := smtp.SendMail(addr, auth, sender, []string{to}, msg)
	if err != nil {
		log.Printf("SMTP send error: %v", err)
	} else {
		log.Printf("Email sent to %s via %s", to, addr)
	}
	return err
}
