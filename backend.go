package smtp

import (
	"github.com/emersion/go-smtp"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Backend implements go-smtp backend interface
type Backend struct {
	plugin *Plugin
	log    *zap.Logger
}

// NewBackend creates SMTP backend
func NewBackend(plugin *Plugin) *Backend {
	return &Backend{
		plugin: plugin,
		log:    plugin.log,
	}
}

// NewSession is called when new SMTP connection is established
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	session := &Session{
		backend:    b,
		conn:       c,
		uuid:       uuid.NewString(),
		remoteAddr: c.Conn().RemoteAddr().String(),
		log:        b.log,
	}

	// Store connection for management
	b.plugin.connections.Store(session.uuid, session)

	b.log.Debug("new SMTP connection",
		zap.String("uuid", session.uuid),
		zap.String("remote_addr", session.remoteAddr),
	)

	return session, nil
}
