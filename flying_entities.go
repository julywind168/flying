package flying

import (
	"fmt"
)

// RegisterEntity 注册实体并启动 worker
func (s *Scheduler) RegisterEntity(e Entity) error {
	if e == nil || e.ID() == "" {
		return fmt.Errorf("invalid entity")
	}

	ectx := &entityContext{
		entity:     e,
		serialChan: make(chan *Event, s.queueSize),
	}

	if _, loaded := s.entities.LoadOrStore(e.ID(), ectx); loaded {
		return fmt.Errorf("entity %s already exists", e.ID())
	}

	s.wg.Add(1)
	go s.serialWorker(ectx)

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
			return fmt.Errorf("Submit `init` to %s failed: %w", e.ID(), err)
		}
	}

	return nil
}

// UnregisterEntity 移除实体，确保在 SubmitAsync 失败时回退并清理
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
		// 防止重复 close：检查 channel 是否已关闭不方便，这里假定只有此处关闭
		close(ectx.serialChan)
		s.notifyObserversByEvent(entityID)
	}

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
				s.logger.Errorf("Submit `exit` to %s failed: %v", entityID, err)
			}
			// fallback: 若投递失败，直接回收，避免实体一直处于 removing 状态
			clearup()
		}
	} else {
		clearup()
	}

	return nil
}

// Observe / Unobserve / notifyObserversByEvent
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
			s.logger.Warnf("failed to notify observer %s: %v", observerID, err)
		}
	}
}
