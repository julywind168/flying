package flying

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Scheduler 主结构体（字段放在单独文件以便拆分）
type Scheduler struct {
	queueSize int

	entities  sync.Map
	observers sync.Map
	pubsub    *pubSubManager

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	logger Logger

	slowEventThreshold time.Duration
	slowEventCallback  func(e *Event, dur, threshold time.Duration)

	totalEventsProcessed atomic.Uint64
	totalDuration        atomic.Uint64 // 纳秒
}

type Option func(*Scheduler)

func WithQueueSize(n int) Option { return func(s *Scheduler) { s.queueSize = n } }
func WithLogger(l Logger) Option { return func(s *Scheduler) { s.logger = l } }
func WithSlowEventThreshold(th time.Duration, cb func(e *Event, dur, threshold time.Duration)) Option {
	return func(s *Scheduler) { s.slowEventThreshold = th; s.slowEventCallback = cb }
}

func New(opts ...Option) *Scheduler {
	s := &Scheduler{
		queueSize: 100,
		pubsub:    newPubSubManager(),
	}
	for _, o := range opts {
		o(s)
	}
	if s.logger == nil {
		s.logger = &defaultLogger{}
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s
}

func (s *Scheduler) Start() {
	if s.logger != nil {
		s.logger.Infof("flying started")
	}
}

func (s *Scheduler) Stop(timeout time.Duration) error {
	s.cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.New("stop timeout")
	}
}
