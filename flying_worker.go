package flying

import (
	"context"
	"fmt"
	"time"
)

func (s *Scheduler) serialWorker(ectx *entityContext) {
	defer s.wg.Done()
	for event := range ectx.serialChan {
		s.processEvent(event, ectx)
	}
}

func (s *Scheduler) processEvent(event *Event, ectx *entityContext) {
	defer s.releaseEventReferences(event, ectx)

	if !event.Deadline.IsZero() && time.Now().After(event.Deadline) {
		if event.Response != nil {
			event.Response <- fmt.Errorf("event timeout before processing")
		}
		return
	}

	start := time.Now()
	defer func() {
		dur := time.Since(start)

		ectx.eventsProcessed.Add(1)
		ectx.totalDuration.Add(uint64(dur.Nanoseconds()))

		s.totalEventsProcessed.Add(1)
		s.totalDuration.Add(uint64(dur.Nanoseconds()))

		if s.slowEventThreshold > 0 && dur > s.slowEventThreshold {
			if s.slowEventCallback != nil {
				s.slowEventCallback(event, dur, s.slowEventThreshold)
			} else if s.logger != nil {
				s.logger.Warnf("slow event: id=%s, entity=%s, type=%s, dur=%v, th=%v",
					event.ID, event.EntityID, event.Type, dur, s.slowEventThreshold)
			}
		}
	}()

	mainVal, ok := s.entities.Load(event.EntityID)
	if !ok {
		if event.Response != nil {
			event.Response <- fmt.Errorf("entity %s not found during processing", event.EntityID)
		}
		return
	}
	mainCtx := mainVal.(*entityContext)

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

	var err error
	if event.Handler != nil {
		err = event.Handler(ctx, mainCtx.entity)
	}

	if event.Response != nil {
		event.Response <- err
	}
}

func (s *Scheduler) releaseEventReferences(event *Event, mainCtx *entityContext) {
	entitiesToRelease := []string{event.EntityID}
	if deps := mainCtx.entity.Dependencies(); len(deps) > 0 {
		entitiesToRelease = append(entitiesToRelease, deps...)
	}
	if event.Dependencies != nil {
		entitiesToRelease = append(entitiesToRelease, event.Dependencies...)
	}
	entitiesToRelease = uniqueAndSort(entitiesToRelease)

	for _, entityID := range entitiesToRelease {
		if val, ok := s.entities.Load(entityID); ok {
			ectx := val.(*entityContext)
			ectx.refCount.Add(-1)
		}
	}
}

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
