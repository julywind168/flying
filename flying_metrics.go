package flying

import "time"

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