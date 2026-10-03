package invoiceemail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	Timeout  time.Duration
}

func (c SMTPConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" || c.Port <= 0 {
		return errors.New("SMTP_HOST y SMTP_PORT son obligatorios")
	}
	if strings.TrimSpace(c.From) == "" {
		return errors.New("EMAIL_FROM es obligatorio")
	}
	from, err := mail.ParseAddress(c.From)
	if err != nil {
		return fmt.Errorf("EMAIL_FROM inválido: %w", err)
	}
	if from.Address != strings.TrimSpace(c.From) {
		return errors.New("EMAIL_FROM debe contener solo la dirección")
	}
	if (c.Username == "") != (c.Password == "") {
		return errors.New("SMTP_USERNAME y SMTP_PASSWORD deben configurarse juntos")
	}
	return nil
}

type SMTPSender struct{ cfg SMTPConfig }

func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &SMTPSender{cfg: cfg}, nil
}

func (s *SMTPSender) Send(ctx context.Context, message Message) error {
	recipient, err := mail.ParseAddress(message.To)
	if err != nil || recipient.Address != strings.TrimSpace(message.To) {
		return errors.New("destinatario de email inválido")
	}
	payload, err := buildMIMEMessage(s.cfg, message)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(s.cfg.Host, fmt.Sprint(s.cfg.Port))
	dialer := &net.Dialer{Timeout: s.cfg.Timeout}
	var conn net.Conn
	if s.cfg.Port == 465 {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.cfg.Timeout))
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if s.cfg.Port != 465 {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return errors.New("el servidor SMTP no ofrece STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(s.cfg.From); err != nil {
		return err
	}
	if err := client.Rcpt(message.To); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func buildMIMEMessage(cfg SMTPConfig, message Message) ([]byte, error) {
	var body bytes.Buffer
	mixed := multipart.NewWriter(&body)
	from := (&mail.Address{Name: sanitizeHeader(cfg.FromName), Address: cfg.From}).String()
	fmt.Fprintf(
		&body,
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%q\r\n\r\n",
		from,
		sanitizeHeader(message.To),
		mime.QEncoding.Encode("utf-8", sanitizeHeader(message.Subject)),
		mixed.Boundary(),
	)

	contentType, content := "text/plain; charset=utf-8", message.Text
	if strings.TrimSpace(message.HTML) != "" {
		contentType, content = "text/html; charset=utf-8", message.HTML
	}
	bodyHeaders := textproto.MIMEHeader{}
	bodyHeaders.Set("Content-Type", contentType)
	bodyHeaders.Set("Content-Transfer-Encoding", "quoted-printable")
	bodyPart, err := mixed.CreatePart(bodyHeaders)
	if err != nil {
		return nil, err
	}
	qp := quotedprintable.NewWriter(bodyPart)
	if _, err := io.WriteString(qp, content); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}

	for _, attachment := range message.Attachments {
		headers := textproto.MIMEHeader{}
		headers.Set("Content-Type", attachment.ContentType)
		headers.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", strings.ReplaceAll(attachment.Filename, "\"", "")))
		headers.Set("Content-Transfer-Encoding", "base64")
		part, err := mixed.CreatePart(headers)
		if err != nil {
			return nil, err
		}
		encoder := base64.NewEncoder(base64.StdEncoding, part)
		if _, err := encoder.Write(attachment.Data); err != nil {
			return nil, err
		}
		if err := encoder.Close(); err != nil {
			return nil, err
		}
	}
	if err := mixed.Close(); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func sanitizeHeader(value string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(value)
}
