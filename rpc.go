package smtp

import (
	"context"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/state/process"
)

// ConnectionInfo represents information about an active SMTP connection
type ConnectionInfo struct {
	UUID          string   `json:"uuid"`
	RemoteAddr    string   `json:"remote_addr"`
	From          string   `json:"from"`
	To            []string `json:"to"`
	Authenticated bool     `json:"authenticated"`
	Username      string   `json:"username"`
}

// rpc provides RPC interface for external management
type rpc struct {
	p *Plugin
}

// AddWorker adds new worker to the pool
func (r *rpc) AddWorker(_ bool, success *bool) error {
	*success = false

	err := r.p.AddWorker()
	if err != nil {
		return err
	}

	*success = true
	return nil
}

// RemoveWorker removes worker from the pool
func (r *rpc) RemoveWorker(_ bool, success *bool) error {
	*success = false

	err := r.p.RemoveWorker(context.Background())
	if err != nil {
		return err
	}

	*success = true
	return nil
}

// WorkersList returns list of active workers
func (r *rpc) WorkersList(_ bool, workers *[]*process.State) error {
	*workers = r.p.Workers()
	return nil
}

// CloseConnection closes SMTP connection by UUID
func (r *rpc) CloseConnection(uuid string, success *bool) error {
	*success = false

	value, ok := r.p.connections.Load(uuid)
	if !ok {
		return errors.Str("connection not found")
	}

	session := value.(*Session)

	// Close underlying connection
	if session.conn != nil && session.conn.Conn() != nil {
		_ = session.conn.Conn().Close()
	}

	r.p.connections.Delete(uuid)
	*success = true

	return nil
}

// ListConnections returns active SMTP connections
func (r *rpc) ListConnections(_ bool, connections *[]ConnectionInfo) error {
	result := make([]ConnectionInfo, 0)

	r.p.connections.Range(func(key, value any) bool {
		session := value.(*Session)
		result = append(result, ConnectionInfo{
			UUID:          session.uuid,
			RemoteAddr:    session.remoteAddr,
			From:          session.from,
			To:            session.to,
			Authenticated: session.authenticated,
			Username:      session.authUsername,
		})
		return true
	})

	*connections = result
	return nil
}
