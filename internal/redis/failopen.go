package redis

import (
	"context"
	"sync/atomic"

	"github.com/kayden-vs/sentinel-proxy/internal/config"
)

type FailOpenClient struct {
	inner     *Client
	available atomic.Bool
	cfg       config.RedisConfig
	onError   func(err error)
}

func NewFailOpenClient(cfg config.RedisConfig, onError func(err error)) *FailOpenClient {
	f := &FailOpenClient{
		cfg:     cfg,
		onError: onError,
	}

	client, err := NewClient(cfg)
	if err != nil {
		f.available.Store(false)
		if onError != nil {
			onError(err)
		}
		return f
	}

	f.inner = client
	f.available.Store(true)
	return f
}

func (f *FailOpenClient) IsAvailable() bool {
	return f.available.Load()
}

func (f *FailOpenClient) RecordRequest(ctx context.Context, userID string, byteCount int64) (*BehaviorStats, error) {
	if !f.available.Load() || f.inner == nil {
		return f.safeDefaults(), nil
	}

	stats, err := f.inner.RecordRequest(ctx, userID, byteCount)
	if err != nil {
		f.handleError(err)
		return f.safeDefaults(), nil
	}
	return stats, nil
}

func (f *FailOpenClient) GetBehaviorStats(ctx context.Context, userID string) (*BehaviorStats, error) {
	if !f.available.Load() || f.inner == nil {
		return f.safeDefaults(), nil
	}

	stats, err := f.inner.GetBehaviorStats(ctx, userID)
	if err != nil {
		f.handleError(err)
		return f.safeDefaults(), nil
	}
	return stats, nil
}

func (f *FailOpenClient) GetViolationCount(ctx context.Context, userID string, windowSec int) (*ViolationRecord, error) {
	if !f.available.Load() || f.inner == nil {
		return &ViolationRecord{}, nil
	}

	record, err := f.inner.GetViolationCount(ctx, userID, windowSec)
	if err != nil {
		f.handleError(err)
		return &ViolationRecord{}, nil
	}
	return record, nil
}

func (f *FailOpenClient) IncrementViolation(ctx context.Context, userID string, windowSec int) (*ViolationRecord, error) {
	if !f.available.Load() || f.inner == nil {
		return &ViolationRecord{Count: 1}, nil
	}

	record, err := f.inner.IncrementViolation(ctx, userID, windowSec)
	if err != nil {
		f.handleError(err)
		return &ViolationRecord{Count: 1}, nil
	}
	return record, nil
}

func (f *FailOpenClient) Ping(ctx context.Context) error {
	if f.inner == nil {
		return nil
	}
	err := f.inner.Ping(ctx)
	if err != nil {
		f.available.Store(false)
	} else {
		f.available.Store(true)
	}
	return err
}

func (f *FailOpenClient) Close() error {
	if f.inner != nil {
		return f.inner.Close()
	}
	return nil
}

func (f *FailOpenClient) safeDefaults() *BehaviorStats {
	return &BehaviorStats{
		IsNewUser: true,
	}
}

func (f *FailOpenClient) handleError(err error) {
	if f.onError != nil {
		f.onError(err)
	}
	if f.inner != nil {
		if pingErr := f.inner.Ping(context.Background()); pingErr != nil {
			f.available.Store(false)
		}
	}
}
