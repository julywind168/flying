package flying

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// 核心接口
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
	OnInit(ctx Context)
	OnExit(ctx Context)
}

type EntityObserver interface {
	OnEntityRemoved(ctx Context, observedID string)
}

// ============================================================================
// 发布订阅接口
// ============================================================================

// EventSubscriber 订阅者接口
type EventSubscriber interface {
	OnEvent(ctx Context, topic string, event any)
}

// EventFilter 事件过滤器
type EventFilter func(event any) bool

// SubscribeOptions 订阅选项
type SubscribeOptions struct {
	Filter   EventFilter // 事件过滤器
	Priority int         // 优先级（数字越大优先级越高）
}

// Context 框架上下文，提供实体管理能力
type Context interface {
	context.Context // 嵌入标准 context

	// Spawn 注册新实体
	Spawn(e Entity) error

	// Remove 移除实体
	Remove(entityID string) error

	// Observe 观察实体
	Observe(observedID string) error

	// Unobserve 取消观察
	Unobserve(observedID string)

	// SendTo 发送消息到指定实体
	SendTo(entityID string, methodName string, payload any) error

	// Publish 发布事件到主题
	Publish(topic string, event any) error

	// Subscribe 订阅主题
	Subscribe(topic string, opts *SubscribeOptions) error

	// Unsubscribe 取消订阅
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

// ============================================================================
// 内部实现
// ============================================================================

// schedulerContext 实现 Context 接口
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

	// 检查实体是否实现了 EventSubscriber
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
// 内部结构
// ============================================================================

type entityContext struct {
	entity     Entity
	serialChan chan *Event
	mu         sync.Mutex
	removing   atomic.Bool
	refCount   atomic.Int32

	// 统计指标
	eventsProcessed atomic.Uint64
	totalDuration   atomic.Uint64 // 纳秒
}

type observerSet struct {
	mu  sync.RWMutex
	set map[string]struct{}
}

// subscription 订阅信息
type subscription struct {
	entityID string
	priority int
	filter   EventFilter
}

// topicSubscribers 主题订阅者集合
type topicSubscribers struct {
	mu          sync.RWMutex
	subscribers []*subscription // 按优先级排序
}

// pubSubManager 发布订阅管理器
type pubSubManager struct {
	topics sync.Map // map[string]*topicSubscribers
}

func newPubSubManager() *pubSubManager {
	return &pubSubManager{}
}

// subscribe 添加订阅
func (pm *pubSubManager) subscribe(topic string, sub *subscription) {
	val, _ := pm.topics.LoadOrStore(topic, &topicSubscribers{
		subscribers: make([]*subscription, 0),
	})

	ts := val.(*topicSubscribers)
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// 插入并保持优先级排序（从高到低）
	inserted := false
	for i, existing := range ts.subscribers {
		if existing.entityID == sub.entityID {
			// 更新已存在的订阅
			ts.subscribers[i] = sub
			inserted = true
			break
		}
		if sub.priority > existing.priority {
			// 插入到合适位置
			ts.subscribers = append(ts.subscribers[:i], append([]*subscription{sub}, ts.subscribers[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		ts.subscribers = append(ts.subscribers, sub)
	}
}

// unsubscribe 取消订阅
func (pm *pubSubManager) unsubscribe(topic string, entityID string) {
	val, ok := pm.topics.Load(topic)
	if !ok {
		return
	}

	ts := val.(*topicSubscribers)
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for i, sub := range ts.subscribers {
		if sub.entityID == entityID {
			ts.subscribers = append(ts.subscribers[:i], ts.subscribers[i+1:]...)
			break
		}
	}
}

// getSubscribers 获取主题的所有订阅者（按优先级排序）
func (pm *pubSubManager) getSubscribers(topic string) []*subscription {
	val, ok := pm.topics.Load(topic)
	if !ok {
		return nil
	}

	ts := val.(*topicSubscribers)
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	// 复制一份避免并发问题
	result := make([]*subscription, len(ts.subscribers))
	copy(result, ts.subscribers)
	return result
}

// ============================================================================
// Scheduler
// ============================================================================

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

	// 全局统计
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

// ============================================================================
// 启动 / 停止
// ============================================================================

func (s *Scheduler) Start() {
	if s.logger != nil {
		s.logger.Infof("flying started\n")
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

// ============================================================================
// 实体管理
// ============================================================================

func (s *Scheduler) RegisterEntity(e Entity) error {
	if e == nil || e.ID() == "" {
		return errors.New("invalid entity")
	}

	ectx := &entityContext{
		entity:     e,
		serialChan: make(chan *Event, s.queueSize),
	}

	if _, loaded := s.entities.LoadOrStore(e.ID(), ectx); loaded {
		return fmt.Errorf("entity %s already exists", e.ID())
	}

	// 启动专属 worker
	s.wg.Add(1)
	go s.serialWorker(ectx)

	// 通过内部事件调用 OnInit
	if lc, ok := e.(EntityLifecycle); ok {
		initEvent := &Event{
			ID:       fmt.Sprintf("internal:init:%s", e.ID()),
			EntityID: e.ID(),
			Type:     "internal:entity-init",
			Handler: func(ctx Context, entity Entity) error {
				lc.OnInit(ctx)
				return nil
			},
		}
		if err := s.SubmitAsync(initEvent); err != nil {
			// 初始化失败，回滚
			s.entities.Delete(e.ID())
			close(ectx.serialChan)
			return fmt.Errorf("Submit `exit` to %s failed: %w", e.ID(), err)
		}
	}

	return nil
}

func (s *Scheduler) UnregisterEntity(entityID string) error {
	val, ok := s.entities.Load(entityID)
	if !ok {
		return fmt.Errorf("entity %s not found", entityID)
	}
	ectx := val.(*entityContext)

	if ectx.removing.Load() {
		return fmt.Errorf("entity %s is already being removed", entityID)
	}

	ectx.removing.Store(true)

	clearup := func() {
		s.entities.Delete(entityID)
		close(ectx.serialChan)
		s.notifyObserversByEvent(entityID)
	}

	// 通过内部事件调用 OnExit
	if lc, ok := ectx.entity.(EntityLifecycle); ok {
		exitEvent := &Event{
			ID:       fmt.Sprintf("internal:exit:%s", entityID),
			EntityID: entityID,
			Type:     "internal:entity-exit",
			Handler: func(ctx Context, entity Entity) error {
				lc.OnExit(ctx)
				clearup()
				return nil
			},
		}
		if err := s.SubmitAsync(exitEvent); err != nil {
			if s.logger != nil {
				s.logger.Errorf("Submit `exit` to %s failed: %v\n", entityID, err)
			}
		}
	} else {
		clearup()
	}

	return nil
}

// ============================================================================
// 事件提交
// ============================================================================

func (s *Scheduler) SubmitAsync(event *Event) error {
	if event == nil || event.EntityID == "" {
		return errors.New("invalid event")
	}

	val, ok := s.entities.Load(event.EntityID)
	if !ok {
		return fmt.Errorf("entity %s not found", event.EntityID)
	}
	ectx := val.(*entityContext)
	if ectx.removing.Load() {
		return fmt.Errorf("entity %s is being removed", event.EntityID)
	}

	// 收集所有需要引用的实体（主实体 + 依赖）
	entitiesToRef := []string{event.EntityID}
	if deps := ectx.entity.Dependencies(); len(deps) > 0 {
		entitiesToRef = append(entitiesToRef, deps...)
	}
	if event.Dependencies != nil {
		entitiesToRef = append(entitiesToRef, event.Dependencies...)
	}
	entitiesToRef = uniqueAndSort(entitiesToRef)

	// 检查所有依赖是否存在并增加引用计数
	acquiredRefs := make([]*entityContext, 0, len(entitiesToRef))
	for _, depID := range entitiesToRef {
		depVal, ok := s.entities.Load(depID)
		if !ok {
			// 依赖不存在，回滚已获取的引用
			for _, acquired := range acquiredRefs {
				acquired.refCount.Add(-1)
			}
			return fmt.Errorf("dependency entity %s not found", depID)
		}
		depCtx := depVal.(*entityContext)
		if depCtx.removing.Load() {
			// 依赖正在被移除，回滚
			for _, acquired := range acquiredRefs {
				acquired.refCount.Add(-1)
			}
			return fmt.Errorf("dependency entity %s is being removed", depID)
		}
		// 增加引用计数
		depCtx.refCount.Add(1)
		acquiredRefs = append(acquiredRefs, depCtx)
	}

	// 投递事件
	select {
	case ectx.serialChan <- event:
		return nil
	case <-s.ctx.Done():
		// 投递失败，释放引用
		for _, acquired := range acquiredRefs {
			acquired.refCount.Add(-1)
		}
		return errors.New("scheduler stopped")
	default:
		// 队列满，释放引用
		for _, acquired := range acquiredRefs {
			acquired.refCount.Add(-1)
		}
		return errors.New("entity queue full")
	}
}

func (s *Scheduler) Submit(event *Event, timeout time.Duration) error {
	if event == nil || event.EntityID == "" {
		return errors.New("invalid event")
	}

	resp := make(chan error, 1)
	event.Response = resp

	if timeout > 0 {
		event.Deadline = time.Now().Add(timeout)
	}

	if err := s.SubmitAsync(event); err != nil {
		close(resp)
		return err
	}

	if timeout <= 0 {
		err := <-resp
		close(resp)
		return err
	}

	select {
	case err := <-resp:
		close(resp)
		return err
	case <-time.After(timeout):
		go func() {
			<-resp
			close(resp)
		}()
		return fmt.Errorf("submit timeout after %v", timeout)
	}
}

// ============================================================================
// Worker
// ============================================================================

func (s *Scheduler) serialWorker(ectx *entityContext) {
	defer s.wg.Done()
	for event := range ectx.serialChan {
		s.processEvent(event, ectx)
	}
}

func (s *Scheduler) processEvent(event *Event, ectx *entityContext) {
	// 在事件处理完成后释放所有引用
	defer s.releaseEventReferences(event, ectx)

	// 检查超时
	if !event.Deadline.IsZero() && time.Now().After(event.Deadline) {
		if event.Response != nil {
			event.Response <- fmt.Errorf("event timeout before processing")
		}
		return
	}

	start := time.Now()
	defer func() {
		dur := time.Since(start)

		// 更新实体统计
		ectx.eventsProcessed.Add(1)
		ectx.totalDuration.Add(uint64(dur.Nanoseconds()))

		// 更新全局统计
		s.totalEventsProcessed.Add(1)
		s.totalDuration.Add(uint64(dur.Nanoseconds()))

		// 慢事件检测
		if s.slowEventThreshold > 0 && dur > s.slowEventThreshold {
			if s.slowEventCallback != nil {
				s.slowEventCallback(event, dur, s.slowEventThreshold)
			} else if s.logger != nil {
				s.logger.Warnf("slow event: id=%s, entity=%s, type=%s, dur=%v, th=%v\n",
					event.ID, event.EntityID, event.Type, dur, s.slowEventThreshold)
			}
		}
	}()

	// 获取主实体（因为引用计数保护，必然存在）
	mainVal, ok := s.entities.Load(event.EntityID)
	if !ok {
		panic(fmt.Sprintf("entity %s not found during processing", event.EntityID))
	}
	mainCtx := mainVal.(*entityContext)

	// 锁定依赖
	depCtxs := s.lockDependencies(event, mainCtx)
	if depCtxs == nil {
		if event.Response != nil {
			event.Response <- fmt.Errorf("failed to lock dependencies for event %s", event.ID)
		}
		return
	}
	defer s.unlockDependencies(depCtxs)

	var ctx Context
	if !event.Deadline.IsZero() {
		remaining := time.Until(event.Deadline)
		if remaining <= 0 {
			if event.Response != nil {
				event.Response <- fmt.Errorf("event timeout during execution")
			}
			return
		}
		stdCtx, cancel := context.WithTimeout(s.ctx, remaining)
		defer cancel()
		ctx = &schedulerContext{
			Context:   stdCtx,
			scheduler: s,
			entityID:  event.EntityID,
		}
	} else {
		ctx = &schedulerContext{
			Context:   s.ctx,
			scheduler: s,
			entityID:  event.EntityID,
		}
	}

	// 执行 handler
	var err error
	if event.Handler != nil {
		err = event.Handler(ctx, mainCtx.entity)
	}

	if event.Response != nil {
		event.Response <- err
	}
}

// releaseEventReferences 释放事件持有的所有实体引用
func (s *Scheduler) releaseEventReferences(event *Event, mainCtx *entityContext) {
	// 收集所有被引用的实体
	entitiesToRelease := []string{event.EntityID}
	if deps := mainCtx.entity.Dependencies(); len(deps) > 0 {
		entitiesToRelease = append(entitiesToRelease, deps...)
	}
	if event.Dependencies != nil {
		entitiesToRelease = append(entitiesToRelease, event.Dependencies...)
	}
	entitiesToRelease = uniqueAndSort(entitiesToRelease)

	// 释放所有引用
	for _, entityID := range entitiesToRelease {
		if val, ok := s.entities.Load(entityID); ok {
			ectx := val.(*entityContext)
			ectx.refCount.Add(-1)
		}
	}
}

// ============================================================================
// 依赖管理
// ============================================================================

func (s *Scheduler) lockDependencies(event *Event, mainCtx *entityContext) []*entityContext {
	ids := []string{event.EntityID}
	if deps := mainCtx.entity.Dependencies(); len(deps) > 0 {
		ids = append(ids, deps...)
	}
	if event.Dependencies != nil {
		ids = append(ids, event.Dependencies...)
	}
	ids = uniqueAndSort(ids)

	locked := make([]*entityContext, 0, len(ids))
	for _, id := range ids {
		val, ok := s.entities.Load(id)
		if !ok {
			s.unlockDependencies(locked)
			return nil
		}
		ectx := val.(*entityContext)
		if !ectx.entity.Lockfree() {
			ectx.mu.Lock()
		}
		locked = append(locked, ectx)
	}
	return locked
}

func (s *Scheduler) unlockDependencies(depContexts []*entityContext) {
	for i := len(depContexts) - 1; i >= 0; i-- {
		ectx := depContexts[i]
		if !ectx.entity.Lockfree() {
			ectx.mu.Unlock()
		}
	}
}

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

// ============================================================================
// Observer
// ============================================================================

func (s *Scheduler) Observe(observerID, observedID string) error {
	val, ok := s.entities.Load(observerID)
	if !ok {
		return fmt.Errorf("observer entity %s not found", observerID)
	}
	_ = val

	v, _ := s.observers.LoadOrStore(observedID, &observerSet{set: make(map[string]struct{})})
	os := v.(*observerSet)
	os.mu.Lock()
	defer os.mu.Unlock()
	os.set[observerID] = struct{}{}
	return nil
}

func (s *Scheduler) Unobserve(observerID, observedID string) {
	v, ok := s.observers.Load(observedID)
	if !ok {
		return
	}
	os := v.(*observerSet)
	os.mu.Lock()
	defer os.mu.Unlock()
	delete(os.set, observerID)
}

func (s *Scheduler) notifyObserversByEvent(observedID string) {
	v, ok := s.observers.Load(observedID)
	if !ok {
		return
	}
	os := v.(*observerSet)

	os.mu.RLock()
	ids := make([]string, 0, len(os.set))
	for id := range os.set {
		ids = append(ids, id)
	}
	os.mu.RUnlock()

	for _, observerID := range ids {
		internalEvt := &Event{
			ID:       fmt.Sprintf("internal:observe:removed:%s->%s", observedID, observerID),
			EntityID: observerID,
			Type:     "internal:entity-removed",
			Handler: func(ctx Context, e Entity) error {
				if ob, ok := e.(EntityObserver); ok {
					ob.OnEntityRemoved(ctx, observedID)
				}
				return nil
			},
		}
		if err := s.SubmitAsync(internalEvt); err != nil && s.logger != nil {
			s.logger.Warnf("failed to notify observer %s: %v\n", observerID, err)
		}
	}
}

// ============================================================================
// 发布实现
// ============================================================================

func (s *Scheduler) Publish(publisherID string, topic string, event any, sync bool) error {
	// 获取所有订阅者
	subscribers := s.pubsub.getSubscribers(topic)
	if len(subscribers) == 0 {
		return nil
	}

	// 收集依赖
	deps := []string{}
	if carrier, ok := event.(EntityCarrier); ok {
		deps = carrier.Dependencies()
	}

	// 给每个订阅者发送事件
	errors := make([]error, 0)
	for _, sub := range subscribers {
		// 应用过滤器
		if sub.filter != nil && !sub.filter(event) {
			continue
		}

		evt := &Event{
			ID:       fmt.Sprintf("pubsub:%s:%s->%s", topic, publisherID, sub.entityID),
			EntityID: sub.entityID,
			Type:     fmt.Sprintf("pubsub:%s", topic),
			Handler: func(ctx Context, e Entity) error {
				if subscriber, ok := e.(EventSubscriber); ok {
					subscriber.OnEvent(ctx, topic, event)
				}
				return nil
			},
			Priority:     sub.priority,
			Dependencies: deps,
		}

		var err error
		if sync {
			err = s.Submit(evt, 0) // 同步等待
		} else {
			err = s.SubmitAsync(evt)
		}

		if err != nil {
			errors = append(errors, fmt.Errorf("failed to publish to %s: %w", sub.entityID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("publish errors: %v", errors)
	}

	return nil
}

// ============================================================================
// 便利方法
// ============================================================================

// PublishToAll 向所有实体广播事件（通配符主题）
func (s *Scheduler) PublishToAll(publisherID string, event any) error {
	return s.Publish(publisherID, "*", event, false)
}

// GetTopicStats 获取主题统计信息
func (s *Scheduler) GetTopicStats() map[string]int {
	stats := make(map[string]int)
	s.pubsub.topics.Range(func(key, val any) bool {
		topic := key.(string)
		ts := val.(*topicSubscribers)
		ts.mu.RLock()
		stats[topic] = len(ts.subscribers)
		ts.mu.RUnlock()
		return true
	})
	return stats
}

// ============================================================================
// Metrics
// ============================================================================

type Metrics struct {
	TotalEntities        int
	TotalEventsProcessed uint64
	AvgEventDuration     time.Duration
	TotalObservers       int
	EntityDetails        map[string]*EntityMetrics
}

type EntityMetrics struct {
	ID               string
	QueueLength      int
	EventsProcessed  uint64
	AvgEventDuration time.Duration
	MemoryUsage      uint64
	RefCount         int32
}

func (s *Scheduler) GetMetrics(details bool) *Metrics {
	m := &Metrics{}

	totalEvents := s.totalEventsProcessed.Load()
	totalDur := s.totalDuration.Load()

	m.TotalEventsProcessed = totalEvents
	if totalEvents > 0 {
		m.AvgEventDuration = time.Duration(totalDur / totalEvents)
	}

	entityCount := 0
	s.entities.Range(func(key, val any) bool {
		entityCount++
		return true
	})
	m.TotalEntities = entityCount

	observerCount := 0
	s.observers.Range(func(key, val any) bool {
		os := val.(*observerSet)
		os.mu.RLock()
		observerCount += len(os.set)
		os.mu.RUnlock()
		return true
	})
	m.TotalObservers = observerCount

	if details {
		m.EntityDetails = make(map[string]*EntityMetrics)

		s.entities.Range(func(key, val any) bool {
			ectx := val.(*entityContext)

			eventsProcessed := ectx.eventsProcessed.Load()
			totalDuration := ectx.totalDuration.Load()

			var avgDur time.Duration
			if eventsProcessed > 0 {
				avgDur = time.Duration(totalDuration / eventsProcessed)
			}

			memUsage := uint64(cap(ectx.serialChan)*128 + 256)

			em := &EntityMetrics{
				ID:               ectx.entity.ID(),
				QueueLength:      len(ectx.serialChan),
				EventsProcessed:  eventsProcessed,
				AvgEventDuration: avgDur,
				MemoryUsage:      memUsage,
				RefCount:         ectx.refCount.Load(),
			}

			m.EntityDetails[ectx.entity.ID()] = em
			return true
		})
	}

	return m
}

// ============================================================================
// Default Logger
// ============================================================================

type defaultLogger struct{}

func (d *defaultLogger) Infof(format string, args ...any)  { fmt.Printf("[I] "+format, args...) }
func (d *defaultLogger) Warnf(format string, args ...any)  { fmt.Printf("[W] "+format, args...) }
func (d *defaultLogger) Errorf(format string, args ...any) { fmt.Printf("[E] "+format, args...) }
