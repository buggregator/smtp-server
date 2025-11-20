package smtp

import (
	"bytes"
	"io"

	"github.com/emersion/go-smtp"
	"go.uber.org/zap"
)

// Session represents an SMTP session (one connection)
type Session struct {
	backend    *Backend
	conn       *smtp.Conn
	uuid       string
	remoteAddr string
	log        *zap.Logger

	// Authentication data (captured but not verified)
	authenticated bool
	authUsername  string
	authPassword  string
	authMechanism string

	// SMTP envelope data
	from     string
	to       []string
	heloName string

	// Email data (accumulated during DATA command)
	emailData bytes.Buffer
}

// Mail is called for MAIL FROM command
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	s.log.Debug("MAIL FROM",
		zap.String("uuid", s.uuid),
		zap.String("from", from),
	)
	return nil
}

// Rcpt is called for RCPT TO command
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	s.log.Debug("RCPT TO",
		zap.String("uuid", s.uuid),
		zap.String("to", to),
	)
	return nil
}

// Data is called when DATA command is received
// Returns error after reading complete email
func (s *Session) Data(r io.Reader) error {
	s.log.Debug("DATA command received", zap.String("uuid", s.uuid))

	// Read email data into buffer
	s.emailData.Reset()
	_, err := io.Copy(&s.emailData, r)
	if err != nil {
		s.log.Error("failed to read email data", zap.Error(err))
		return err
	}

	s.log.Info("email received",
		zap.String("uuid", s.uuid),
		zap.String("from", s.from),
		zap.Strings("to", s.to),
		zap.Int("size", s.emailData.Len()),
	)

	// Step 4 will send this data to PHP workers
	// For now, just log it

	return nil
}

// Reset is called for RSET command
func (s *Session) Reset() {
	s.from = ""
	s.to = nil
	s.emailData.Reset()
	s.log.Debug("session reset", zap.String("uuid", s.uuid))
}

// Logout is called when connection closes
func (s *Session) Logout() error {
	s.log.Debug("connection closed", zap.String("uuid", s.uuid))
	s.backend.plugin.connections.Delete(s.uuid)
	return nil
}
