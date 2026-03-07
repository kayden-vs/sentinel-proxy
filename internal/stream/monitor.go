package stream

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kayden-vs/sentinel-proxy/internal/config"
	"github.com/kayden-vs/sentinel-proxy/internal/metrics"
	"github.com/kayden-vs/sentinel-proxy/internal/policy"
	redisclient "github.com/kayden-vs/sentinel-proxy/internal/redis"
	"github.com/kayden-vs/sentinel-proxy/internal/threshold"
	pb "github.com/kayden-vs/sentinel-proxy/proto/sentinel"
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

type StreamResult struct {
	Outcome    string
	Killed     bool
	KillReason string
	TotalBytes int64
	ChunkCount int64
	Duration   time.Duration
	Throttled  bool
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

func (m *Monitor) Stream(
	ctx context.Context,
	grpcStream pb.DataService_GetDataClient,
	w http.ResponseWriter,
) *StreamResult {
	m.startTime = time.Now()
	m.m.ActiveStreams.Inc()
	defer m.m.ActiveStreams.Dec()

	result := &StreamResult{}
	flusher, hasFlusher := w.(http.Flusher)

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Sentinel-User", m.userID)
	w.Header().Set("X-Sentinel-Threshold", fmt.Sprintf("%d", m.decision.Allowed))
	w.Header().Set("X-Sentinel-Ceiling", fmt.Sprintf("%d", m.decision.AbsoluteCeiling))

	clientGone := ctx.Done()

	for {
		select {
		case <-clientGone:
			m.logger.Warn("client disconnected mid-stream",
				"user_id", m.userID,
				"bytes_sent", m.totalBytes.Load(),
				"chunks_sent", m.chunkCount.Load(),
			)
			result.Outcome = "client_disconnect"
			m.finalize(result)
			return result

		default:
			chunk, err := grpcStream.Recv()
			if err != nil {
				if err == io.EOF {
					result.Outcome = "complete"
					m.finalize(result)
					return result
				}

				if m.killed.Load() {
					result.Outcome = "killed"
					m.finalize(result)
					return result
				}

				m.logger.Error("gRPC stream error",
					"user_id", m.userID,
					"error", err,
					"bytes_sent", m.totalBytes.Load(),
				)
				result.Outcome = "grpc_error"
				m.finalize(result)
				return result
			}

			chunkSize := int64(len(chunk.Payload))
			newTotal := m.totalBytes.Add(chunkSize)
			m.chunkCount.Add(1)

			m.lastChunkTimeMu.Lock()
			m.lastChunkTime = time.Now()
			m.lastChunkTimeMu.Unlock()

			elapsed := time.Since(m.startTime).Seconds()
			var currentRateBPS float64

			minElapsed := float64(m.cfg.Threshold.MinRateElapsedMs) / 1000.0
			if minElapsed <= 0 {
				minElapsed = 0.5 // 500ms default safety
			}
			if elapsed >= minElapsed {
				currentRateBPS = float64(newTotal) / elapsed
			}

			updatedDecision := m.engine.Evaluate(
				&redisclient.BehaviorStats{
					AverageBytes:   m.decision.HistoricalAvg,
					AverageRateBPS: m.decision.HistoricalAvg / 10.0,
					RequestCount:   10,
				},
				m.endpoint,
				m.role,
				m.cfg.Policies,
				currentRateBPS,
			)
			m.decision.AnomalyDetected = updatedDecision.AnomalyDetected

			// hard breach: absolute ceiling exceeded: terminate conn
			// SofT breach: adaptive threshold exceeded: throttle
			shouldKill, breachType, reason := m.engine.ShouldKill(newTotal, m.decision)
			if shouldKill {
				if breachType == threshold.BreachHard {
					// HARD BREACH: immediate termination, bypass grace system entirely
					m.hardKill(w, reason, result)
					return result
				}
				// SOFT BREACH: evaluate graduated enforcement (once per stream)
				if !m.softBreachHandled.Load() {
					m.softBreachHandled.Store(true)
					action := m.evaluateSoftBreach(ctx, reason)
					switch action {
					case enforceContinue:
						m.throttled.Store(false)
					case enforceThrottle:
						m.throttled.Store(true)
					case enforceTerminate:
						m.hardKill(w, reason+" (graduated enforcement)", result)
						return result
					}
				}
			}

			if m.engine.ShouldThrottle(newTotal, m.decision) && !m.throttled.Load() {
				m.throttled.Store(true)
				m.logger.Warn("STREAM THROTTLED",
					"user_id", m.userID,
					"total_bytes", newTotal,
					"allowed", m.decision.Allowed,
					"endpoint", m.endpoint,
				)
				m.m.ThresholdDecisions.WithLabelValues("throttle").Inc()
			}

			_, writeErr := w.Write(chunk.Payload)
			if writeErr != nil {
				m.logger.Warn("failed to write to client",
					"user_id", m.userID,
					"error", writeErr,
				)
				result.Outcome = "write_error"
				m.finalize(result)
				return result
			}

			// FIX: Adaptive throttle delay - scales with how far over threshold
			if m.throttled.Load() {
				delay := m.adaptiveDelay(newTotal)
				time.Sleep(delay)
			}

			if hasFlusher {
				flusher.Flush()
			}

			m.m.BytesStreamed.WithLabelValues(m.userID, m.endpoint).Add(float64(chunkSize))

			if m.chunkCount.Load()%100 == 0 {
				m.logger.Debug("stream progress",
					"user_id", m.userID,
					"chunks", m.chunkCount.Load(),
					"total_bytes", newTotal,
					"allowed", m.decision.Allowed,
					"rate_bps", currentRateBPS,
					"anomaly", m.decision.AnomalyDetected,
				)
			}

			if chunk.IsLast {
				result.Outcome = "complete"
				m.finalize(result)
				return result
			}
		}
	}
}

type enforceAction int

const (
	enforceContinue  enforceAction = iota // log only, continue streaming
	enforceThrottle                       // throttle, continue streaming
	enforceTerminate                      // terminate stream
)

// hardKill immediately terminates the stream
func (m *Monitor) hardKill(w http.ResponseWriter, reason string, result *StreamResult) {
	m.killed.Store(true)

	m.logger.Error("STREAM HARD KILL",
		"user_id", m.userID,
		"reason", reason,
		"total_bytes", m.totalBytes.Load(),
		"allowed", m.decision.Allowed,
		"ceiling", m.decision.AbsoluteCeiling,
		"endpoint", m.endpoint,
		"chunks_sent", m.chunkCount.Load(),
		"duration", time.Since(m.startTime),
		"anomaly_detected", m.decision.AnomalyDetected,
	)

	if m.cancel != nil {
		m.cancel()
	}

	m.m.StreamKillsTotal.WithLabelValues(reason, m.endpoint).Inc()
	m.m.ThresholdDecisions.WithLabelValues("kill").Inc()

	if m.decision.AnomalyDetected {
		m.m.AnomaliesDetected.WithLabelValues(m.userID, m.endpoint).Inc()
	}

	result.Killed = true
	result.KillReason = reason
	result.Outcome = "killed"
	m.finalize(result)
}

func (m *Monitor) evaluateSoftBreach(ctx context.Context, reason string) enforceAction {
	m.logger.Warn("SOFT BREACH detected",
		"user_id", m.userID,
		"reason", reason,
		"total_bytes", m.totalBytes.Load(),
		"allowed", m.decision.Allowed,
		"endpoint", m.endpoint,
	)

	grade, violation, err := m.enforcer.Evaluate(ctx, m.userID)
	if err != nil {
		m.logger.Error("failed to evaluate enforcement grade", "error", err)
	}

	m.m.ViolationsByGrade.WithLabelValues(grade.String()).Inc()

	m.logger.Warn("enforcement decision",
		"user_id", m.userID,
		"grade", grade.String(),
		"violation_count", violation.Count,
		"reason", reason,
	)

	switch grade {
	case policy.GradeLogOnly:
		m.logger.Warn("VIOLATION LOGGED (grace period)",
			"user_id", m.userID,
			"violation_count", violation.Count,
		)
		m.m.ThresholdDecisions.WithLabelValues("grace_log").Inc()
		return enforceContinue

	case policy.GradeThrottle:
		m.logger.Warn("STREAM THROTTLED (repeated violation)",
			"user_id", m.userID,
			"violation_count", violation.Count,
		)
		m.m.ThresholdDecisions.WithLabelValues("grace_throttle").Inc()
		return enforceThrottle

	default:
		m.logger.Error("STREAM TERMINATED (graduated enforcement)",
			"user_id", m.userID,
			"violation_count", violation.Count,
		)
		return enforceTerminate
	}
}

// adaptiveDelay computes throttle delay
func (m *Monitor) adaptiveDelay(currentBytes int64) time.Duration {
	baseMs := m.cfg.Threshold.BaseThrottleDelayMs
	maxMs := m.cfg.Threshold.MaxThrottleDelayMs
	if baseMs <= 0 {
		baseMs = 20
	}
	if maxMs <= 0 {
		maxMs = 500
	}

	if m.decision.Allowed <= 0 {
		return time.Duration(baseMs) * time.Millisecond
	}

	ratio := float64(currentBytes) / float64(m.decision.Allowed)
	delayMs := float64(baseMs) * ratio

	if delayMs > float64(maxMs) {
		delayMs = float64(maxMs)
	}
	if delayMs < float64(baseMs) {
		delayMs = float64(baseMs)
	}

	return time.Duration(delayMs) * time.Millisecond
}

func (m *Monitor) finalize(result *StreamResult) {
	result.TotalBytes = m.totalBytes.Load()
	result.ChunkCount = m.chunkCount.Load()
	result.Duration = time.Since(m.startTime)
	result.Throttled = m.throttled.Load()

	if result.TotalBytes > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := m.redis.RecordRequest(ctx, m.userID, result.TotalBytes)
		if err != nil {
			m.logger.Error("failed to record request to Redis",
				"user_id", m.userID,
				"error", err,
			)
		}
	}

	outcome := result.Outcome
	m.m.StreamDuration.WithLabelValues(m.endpoint, outcome).Observe(result.Duration.Seconds())
	m.m.BytesPerRequest.WithLabelValues(m.endpoint).Observe(float64(result.TotalBytes))
}
