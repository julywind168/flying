package flying

import (
	"container/list"
	"log"
	"time"
)

const (
	SERVICE_MIN_TICK_DURATION = 10 * time.Millisecond
)

type commandAddService struct {
	service *Service[UService]
}

type commandTick struct {
	delta time.Duration
}

type commandFireEvent struct {
	event Event
}

type commandEventDone struct {
	service    *Service[UService]
	waitUnlock []Node
}

type commandShutdown struct {
	done chan struct{}
}

type World struct {
	services      map[string]*Service[UService]
	commands      chan any
	events        *list.List
	stopTimerChan chan struct{}
}

func NewWorld() *World {
	return &World{
		services:      make(map[string]*Service[UService]),
		commands:      make(chan any, 1024),
		events:        list.New(),
		stopTimerChan: make(chan struct{}),
	}
}

func Spawn[T UService](w *World, id string, tickInterval time.Duration, state T) {
	if tickInterval < SERVICE_MIN_TICK_DURATION {
		log.Panicf("Service tick interval cannot be less than %v", SERVICE_MIN_TICK_DURATION)
	}
	service := &Service[UService]{
		BaseNode: *NewBaseNode(id),
		World:    w,
		Timer: Timer{
			Interval: tickInterval,
			Elapsed:  0,
		},
		State: state,
	}

	w.commands <- commandAddService{service}
	w.commands <- commandFireEvent{Event{
		From: "",
		To:   service.ID(),
		Type: EventTypeStarted,
	}}
}

func (w *World) doEvent(s *Service[UService], e Event) {
	group := append(s.Children(), e.Children()...)
	for _, node := range group {
		node.SetLocked(true)
	}
	go func() {
		switch e.Type {
		case EventTypeStarted:
			s.Started()
		case EventTypeTick:
			s.Tick()
		case EventTypeMessage:
			s.Message(e.From, e.Playload.(Message))
		case EventTypeClientReq:
			session := e.Children()[0].(ISession)
			s.Handler(session, e.Playload.(Message))
		}
		w.commands <- commandEventDone{
			service:    s,
			waitUnlock: group,
		}
	}()
}

func (w *World) tryDispatchPendingEvents() {
	for e := w.events.Front(); e != nil; e = e.Next() {
		event := e.Value.(Event)
		if s, exist := w.services[event.To]; exist {
			if s.IsReady() && event.IsReady() {
				w.events.Remove(e)
				w.doEvent(s, event)
				w.tryDispatchPendingEvents()
			} else {
				continue
			}
		} else {
			w.events.Remove(e)
			log.Printf("Service<%s> does not exist, event %+v discarded", event.To, event)
			w.tryDispatchPendingEvents()
		}
	}
}

func (w *World) tick(dt time.Duration) {
	for _, service := range w.services {
		if service.Timer.Tick(dt).JustFinished() {
			w.commands <- commandFireEvent{
				event: Event{
					From: "",
					To:   service.ID(),
					Type: EventTypeTick,
				},
			}
		}
	}
}

func (w *World) FireClientRequest(to string, session ISession, name string, params any) {
	w.commands <- commandFireEvent{
		Event{
			BaseNode: BaseNode{
				children: []Node{session},
			},
			From: "",
			To:   to,
			Type: EventTypeClientReq,
			Playload: Message{
				Name:   name,
				Params: params,
			},
		},
	}
}

func (w *World) FireServiceMessage(from string, to string, name string, params any) {
	w.commands <- commandFireEvent{
		event: Event{
			From: from,
			To:   to,
			Type: EventTypeMessage,
			Playload: Message{
				Name:   name,
				Params: params,
			},
		},
	}
}

func (w *World) Start() {
	go func() {
		// timer thread
		go func() {
			var (
				last = time.Now()
				dt   = time.Duration(0)
			)
			for {
				select {
				case <-w.stopTimerChan:
					return
				case <-time.After(1 * time.Millisecond):
					// pass
				}
				now := time.Now()
				dt = now.Sub(last)
				last = now
				w.commands <- commandTick{delta: dt}
			}
		}()

		// mainloop
		for cmd := range w.commands {
			switch c := cmd.(type) {
			case commandAddService:
				w.services[c.service.ID()] = c.service
			case commandTick:
				w.tick(c.delta)
			case commandFireEvent:
				if service, exist := w.services[c.event.To]; exist {
					if service.IsReady() && c.event.IsReady() {
						w.doEvent(service, c.event)
					} else {
						w.events.PushBack(c.event)
					}
				} else {
					log.Printf("Service<%s> does not exist, event %+v discarded", c.event.To, c.event)
				}
			case commandEventDone:
				for _, node := range c.waitUnlock {
					node.SetLocked(false)
				}
				if c.service.Exited {
					delete(w.services, c.service.ID())
					log.Printf("Service<%s> exited", c.service.ID())
				}
				w.tryDispatchPendingEvents()
			case commandShutdown:
				close(w.stopTimerChan)
				close(c.done)
				return
			}
		}
	}()
}

func (w *World) Stop() {
	done := make(chan struct{})
	w.commands <- commandShutdown{done}
	<-done
}
