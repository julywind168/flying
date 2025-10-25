package flying

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// 核心接口 & 基础类型
// ============================================================================

type Entity interface {
	ID() string
	Lockfree() bool
	Dependencies() []string
}

type EntityCarrier interface {
	Dependencies() []string
}

type EntityLifecycle interface {
	Init(ctx Context)
	Exit(ctx Context)
}

type EntityObserver interface {
	HandleEntityRemoved(ctx Context, observedID string)
}

type EventSubscriber interface {
	OnEvent(ctx Context, topic string, event any)
}

type EventFilter func(event any) bool

type SubscribeOptions struct {
	Filter   EventFilter
	Priority int
}

type Context interface {
	context.Context
	Spawn(e Entity) error
	Remove(entityID string) error
	Observe(observedID string) error
	Unobserve(observedID string)
	SendTo(entityID string, methodName string, payload any) error
	Publish(topic string, event any) error
	Subscribe(topic string, opts *SubscribeOptions) error
	Unsubscribe(topic string)
}

type Event struct {
	ID           string
	EntityID     string
	Type         string
	Handler      func(ctx Context, e Entity) error
	Priority     int
	Dependencies []string
	Response     chan error
	Deadline     time.Time
}

type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// internal scheduler context
type schedulerContext struct {
	context.Context
	scheduler *Scheduler
	entityID  string
}

func (sc *schedulerContext) Spawn(e Entity) error {
	return sc.scheduler.RegisterEntity(e)
}

func (sc *schedulerContext) Remove(entityID string) error {
	return sc.scheduler.UnregisterEntity(entityID)
}

func (sc *schedulerContext) Submit(event *Event, timeout time.Duration) error {
	return sc.scheduler.Submit(event, timeout)
}

func (sc *schedulerContext) SubmitAsync(event *Event) error {
	return sc.scheduler.SubmitAsync(event)
}

func (sc *schedulerContext) SendTo(entityID string, methodName string, payload any) error {
	deps := []string{}

	if carrier, ok := payload.(EntityCarrier); ok {
		deps = carrier.Dependencies()
	}

	event := &Event{
		ID:       fmt.Sprintf("sendto:%s->%s", sc.entityID, entityID),
		EntityID: entityID,
		Type:     "internal:send-to",
		Handler: func(ctx Context, e Entity) error {
			v := reflect.ValueOf(e)
			m := v.MethodByName(methodName)
			if !m.IsValid() {
				return fmt.Errorf("entity %s does not have method %s", e.ID(), methodName)
			}
			mTy := m.Type()
			args := []reflect.Value{reflect.ValueOf(sc.entityID), reflect.ValueOf(payload)}
			if !reflect.TypeOf(sc.entityID).AssignableTo(mTy.In(0)) ||
				!reflect.TypeOf(payload).AssignableTo(mTy.In(1)) {
				return fmt.Errorf("entity %s method %s does not have the correct signature", e.ID(), methodName)
			}
			m.Call(args)
			return nil
		},
		Dependencies: deps,
	}
	return sc.scheduler.SubmitAsync(event)
}

func (sc *schedulerContext) Observe(observedID string) error {
	return sc.scheduler.Observe(sc.entityID, observedID)
}

func (sc *schedulerContext) Unobserve(observedID string) {
	sc.scheduler.Unobserve(sc.entityID, observedID)
}

func (sc *schedulerContext) Publish(topic string, event any) error {
	return sc.scheduler.Publish(sc.entityID, topic, event, false)
}

func (sc *schedulerContext) Subscribe(topic string, opts *SubscribeOptions) error {
	if opts == nil {
		opts = &SubscribeOptions{}
	}

	val, ok := sc.scheduler.entities.Load(sc.entityID)
	if !ok {
		return fmt.Errorf("entity %s not found", sc.entityID)
	}
	ectx := val.(*entityContext)

	if _, ok := ectx.entity.(EventSubscriber); !ok {
		return fmt.Errorf("entity %s does not implement EventSubscriber", sc.entityID)
	}

	sub := &subscription{
		entityID: sc.entityID,
		priority: opts.Priority,
		filter:   opts.Filter,
	}

	sc.scheduler.pubsub.subscribe(topic, sub)
	return nil
}

func (sc *schedulerContext) Unsubscribe(topic string) {
	sc.scheduler.pubsub.unsubscribe(topic, sc.entityID)
}

// ============================================================================
// internal structures
// ============================================================================

type entityContext struct {
	entity     Entity
	serialChan chan *Event
	mu         sync.Mutex
	removing   atomic.Bool
	refCount   atomic.Int32

	eventsProcessed atomic.Uint64
	totalDuration   atomic.Uint64 // 纳秒
}

type observerSet struct {
	mu  sync.RWMutex
	set map[string]struct{}
}

type subscription struct {
	entityID string
	priority int
	filter   EventFilter
}

type topicSubscribers struct {
	mu          sync.RWMutex
	subscribers []*subscription // 按优先级排序
}

type pubSubManager struct {
	topics sync.Map // map[string]*topicSubscribers
}

func newPubSubManager() *pubSubManager {
	return &pubSubManager{}
}

// unique helper
func uniqueAndSort(ids []string) []string {
	if len(ids) == 0 {
		return ids
	}
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	res := make([]string, 0, len(m))
	for id := range m {
		res = append(res, id)
	}
	sort.Strings(res)
	return res
}
