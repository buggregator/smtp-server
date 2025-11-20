package smtp

import (
	"context"
	"sync"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/payload"
	"github.com/roadrunner-server/pool/pool"
	staticPool "github.com/roadrunner-server/pool/pool/static_pool"
	"github.com/roadrunner-server/pool/state/process"
	"github.com/roadrunner-server/pool/worker"
	"go.uber.org/zap"
)

const (
	PluginName = "smtp"
	RrMode     = "RR_MODE"
)

// Pool interface for worker pool operations
type Pool interface {
	// Workers returns worker list associated with the pool
	Workers() (workers []*worker.Process)
	// RemoveWorker removes worker from the pool
	RemoveWorker(ctx context.Context) error
	// AddWorker adds worker to the pool
	AddWorker() error
	// Exec executes payload
	Exec(ctx context.Context, p *payload.Payload, stopCh chan struct{}) (chan *staticPool.PExec, error)
	// Reset kills all workers and replaces with new
	Reset(ctx context.Context) error
	// Destroy all underlying stacks
	Destroy(ctx context.Context)
}

// Logger interface for dependency injection
type Logger interface {
	NamedLogger(name string) *zap.Logger
}

// Server creates workers for the application
type Server interface {
	NewPool(ctx context.Context, cfg *pool.Config, env map[string]string, _ *zap.Logger) (*staticPool.Pool, error)
}

// Configurer interface for configuration access
type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct
	UnmarshalKey(name string, out any) error
	// Has checks if a config section exists
	Has(name string) bool
}

// Plugin is the SMTP server plugin
type Plugin struct {
	mu     sync.RWMutex
	cfg    *Config
	log    *zap.Logger
	server Server

	wPool       Pool
	connections sync.Map // uuid -> conn
	pldPool     sync.Pool
}

// Init initializes the plugin with configuration and logger
func (p *Plugin) Init(log Logger, cfg Configurer, server Server) error {
	const op = errors.Op("smtp_plugin_init")

	// Check if plugin is enabled
	if !cfg.Has(PluginName) {
		return errors.E(op, errors.Disabled)
	}

	// Parse configuration
	err := cfg.UnmarshalKey(PluginName, &p.cfg)
	if err != nil {
		return errors.E(op, err)
	}

	// Initialize defaults
	if err := p.cfg.InitDefaults(); err != nil {
		return errors.E(op, err)
	}

	// Initialize payload pool
	p.pldPool = sync.Pool{
		New: func() any {
			return new(payload.Payload)
		},
	}

	// Setup logger
	p.log = log.NamedLogger(PluginName)
	p.server = server

	p.log.Info("SMTP plugin initialized",
		zap.String("addr", p.cfg.Addr),
		zap.String("hostname", p.cfg.Hostname),
		zap.Int64("max_message_size", p.cfg.MaxMessageSize),
	)

	return nil
}

// Serve starts the SMTP server
func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 1)

	// Create worker pool
	var err error
	p.wPool, err = p.server.NewPool(context.Background(), p.cfg.Pool, map[string]string{RrMode: PluginName}, nil)
	if err != nil {
		errCh <- err
		return errCh
	}

	p.log.Info("SMTP server starting", zap.String("addr", p.cfg.Addr))

	// TODO: Start SMTP server listener in next step

	return errCh
}

// Stop gracefully stops the plugin
func (p *Plugin) Stop(ctx context.Context) error {
	p.log.Info("SMTP server stopping")

	doneCh := make(chan struct{}, 1)

	go func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		// Close all connections
		p.connections.Range(func(_, value any) bool {
			// TODO: Close connection in next step
			return true
		})

		// Destroy worker pool
		if p.wPool != nil {
			switch pp := p.wPool.(type) {
			case *staticPool.Pool:
				if pp != nil {
					pp.Destroy(ctx)
				}
			}
		}

		doneCh <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		return nil
	}
}

// Reset resets the worker pool
func (p *Plugin) Reset() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	const op = errors.Op("smtp_reset")
	p.log.Info("reset signal was received")

	err := p.wPool.Reset(context.Background())
	if err != nil {
		return errors.E(op, err)
	}

	p.log.Info("plugin was successfully reset")
	return nil
}

// Workers returns the state of all workers
func (p *Plugin) Workers() []*process.State {
	p.mu.RLock()
	wrk := p.wPool.Workers()
	p.mu.RUnlock()

	ps := make([]*process.State, len(wrk))

	for i := range wrk {
		st, err := process.WorkerProcessState(wrk[i])
		if err != nil {
			p.log.Error("smtp workers state", zap.Error(err))
			return nil
		}
		ps[i] = st
	}

	return ps
}

// Name returns plugin name for RoadRunner
func (p *Plugin) Name() string {
	return PluginName
}

// RPC returns the RPC interface
func (p *Plugin) RPC() any {
	return &rpc{
		p: p,
	}
}

// rpc is a placeholder for RPC methods
type rpc struct {
	p *Plugin
}
