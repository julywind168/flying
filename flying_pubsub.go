package flying

import "fmt"

func (pm *pubSubManager) subscribe(topic string, sub *subscription) {
	val, _ := pm.topics.LoadOrStore(topic, &topicSubscribers{
		subscribers: make([]*subscription, 0),
	})

	ts := val.(*topicSubscribers)
	ts.mu.Lock()
	defer ts.mu.Unlock()

	inserted := false
	for i, existing := range ts.subscribers {
		if existing.entityID == sub.entityID {
			ts.subscribers[i] = sub
			inserted = true
			break
		}
		if sub.priority > existing.priority {
			ts.subscribers = append(ts.subscribers[:i], append([]*subscription{sub}, ts.subscribers[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		ts.subscribers = append(ts.subscribers, sub)
	}
}

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

func (pm *pubSubManager) getSubscribers(topic string) []*subscription {
	val, ok := pm.topics.Load(topic)
	if !ok {
		return nil
	}

	ts := val.(*topicSubscribers)
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]*subscription, len(ts.subscribers))
	copy(result, ts.subscribers)
	return result
}

func (s *Scheduler) Publish(publisherID string, topic string, event any, sync bool) error {
	subscribers := s.pubsub.getSubscribers(topic)
	if len(subscribers) == 0 {
		return nil
	}

	deps := []string{}
	if carrier, ok := event.(EntityCarrier); ok {
		deps = carrier.Dependencies()
	}

	errorsList := make([]error, 0)
	for _, sub := range subscribers {
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
			err = s.Submit(evt, 0)
		} else {
			err = s.SubmitAsync(evt)
		}

		if err != nil {
			errorsList = append(errorsList, fmt.Errorf("failed to publish to %s: %w", sub.entityID, err))
		}
	}

	if len(errorsList) > 0 {
		return fmt.Errorf("publish errors: %v", errorsList)
	}
	return nil
}

func (s *Scheduler) PublishToAll(publisherID string, event any) error {
	return s.Publish(publisherID, "*", event, false)
}

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
