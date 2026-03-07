package stream

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kayden-vs/sentinel-proxy/internal/config"
	"github.com/kayden-vs/sentinel-proxy/internal/metrics"
	"github.com/kayden-vs/sentinel-proxy/internal/policy"
	redisclient "github.com/kayden-vs/sentinel-proxy/internal/redis"
	"github.com/kayden-vs/sentinel-proxy/internal/threshold"
)

type Monitor struct {
	userID   string
	endpoint string
	role     string
	decision *threshold.Decision
	engine   *threshold.Engine
	enforcer *policy.Enforcer
	redis    *redisclient.FailOpenClient
	cfg      config.Config
	logger   *slog.Logger
	m        *metrics.Metrics

	totalBytes        atomic.Int64
	chunkCount        atomic.Int64
	startTime         time.Time
	lastChunkTime     time.Time
	lastChunkTimeMu   sync.Mutex
	killed            atomic.Bool
	throttled         atomic.Bool
	softBreachHandled atomic.Bool // prevents repeated grace evaluation per stream

	cancel context.CancelFunc
}

type MonitorConfig struct {
	UserID   string
	Endpoint string
	Role     string
	Decision *threshold.Decision
	Engine   *threshold.Engine
	Enforcer *policy.Enforcer
	Redis    *redisclient.FailOpenClient
	Config   config.Config
	Logger   *slog.Logger
	Cancel   context.CancelFunc
}

func NewMonitor(mc MonitorConfig) *Monitor {
	return &Monitor{
		userID:   mc.UserID,
		endpoint: mc.Endpoint,
		role:     mc.Role,
		decision: mc.Decision,
		engine:   mc.Engine,
		enforcer: mc.Enforcer,
		redis:    mc.Redis,
		cfg:      mc.Config,
		logger:   mc.Logger,
		m:        metrics.Get(),
		cancel:   mc.Cancel,
	}
}
