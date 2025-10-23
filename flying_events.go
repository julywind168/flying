package flying

import (
	"errors"
	"fmt"
	"time"
)

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

	entitiesToRef := []string{event.EntityID}
	if deps := ectx.entity.Dependencies(); len(deps) > 0 {
		entitiesToRef = append(entitiesToRef, deps...)
	}
	if event.Dependencies != nil {
		entitiesToRef = append(entitiesToRef, event.Dependencies...)
	}
	entitiesToRef = uniqueAndSort(entitiesToRef)

	acquiredRefs := make([]*entityContext, 0, len(entitiesToRef))
	for _, depID := range entitiesToRef {
		depVal, ok := s.entities.Load(depID)
		if !ok {
			for _, acquired := range acquiredRefs {
				acquired.refCount.Add(-1)
			}
			return fmt.Errorf("dependency entity %s not found", depID)
		}
		depCtx := depVal.(*entityContext)
		if depCtx.removing.Load() {
			for _, acquired := range acquiredRefs {
				acquired.refCount.Add(-1)
			}
			return fmt.Errorf("dependency entity %s is being removed", depID)
		}
		depCtx.refCount.Add(1)
		acquiredRefs = append(acquiredRefs, depCtx)
	}

	select {
	case ectx.serialChan <- event:
		return nil
	case <-s.ctx.Done():
		for _, acquired := range acquiredRefs {
			acquired.refCount.Add(-1)
		}
		return errors.New("scheduler stopped")
	default:
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
