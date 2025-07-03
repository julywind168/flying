package main

import (
	"container/list"
	"fmt"
	"log"
	"time"
)

const (
	SERVICE_MIN_TICK_DURATION = 10 * time.Millisecond
)

type Node interface {
	ID() string
	AddChild(child Node)
	RemoveChild(child Node) bool
	ChildCount() int
	Children() []Node
	SetLocked(bool)
	IsReady() bool
	IsLeaf() bool
}

type Timer struct {
	Elapsed  time.Duration
	Interval time.Duration
}

func (t *Timer) Tick(delta time.Duration) *Timer {
	t.Elapsed += delta
	return t
}

func (t *Timer) Reset() {
	t.Elapsed = 0
}

func (t *Timer) JustFinished() bool {
	if t.Elapsed >= t.Interval {
		t.Elapsed -= t.Interval
		return true
	} else {
		return false
	}
}

type IServiceState interface {
	Started(ctx ServiceCtx)
	Tick(ctx ServiceCtx, dt time.Duration)
	Handler(ctx ServiceCtx)
	Stopped(ctx ServiceCtx)
}

type IService interface {
	Node
	Started()
	Tick(dt time.Duration)
	Handler()
	Stopped()
}

type EventType uint8

const (
	EventTypeStarted EventType = iota
	EventTypeTick
	EventTypeClientReq
)

type Event struct {
	BaseNode
	Type     EventType
	From     string
	To       string
	Playload any
}

type State[T any] struct {
	BaseNode
	Value T
}

type BaseNode struct {
	name     string
	locked   bool
	children []Node
}

type Service[T IServiceState] struct {
	BaseNode
	Timer        Timer
	State        T
	StartTime    time.Time
	LastTickTime time.Time
	Exited       bool
}

type ServiceCtx interface {
	self() string
	exit()
}

type internalServiceCtx struct {
	selfFunc func() string
	exitFunc func()
}

func (c *internalServiceCtx) self() string {
	return c.selfFunc()
}

func (c *internalServiceCtx) exit() {
	c.exitFunc()
}

func (s *Service[T]) getCtx() ServiceCtx {
	return &internalServiceCtx{
		selfFunc: s.ID,
		exitFunc: func() {
			if !s.Exited {
				s.Exited = true
				s.Stopped()
			}
		},
	}
}

func NewService[T IServiceState](name string, tickInterval time.Duration, state T) *Service[IServiceState] {
	if tickInterval < SERVICE_MIN_TICK_DURATION {
		log.Panicf("Service tick interval cannot be less than %v", SERVICE_MIN_TICK_DURATION)
	}

	return &Service[IServiceState]{
		BaseNode: *NewBaseNode(name),
		Timer: Timer{
			Interval: tickInterval,
			Elapsed:  0,
		},
		State:        state,
		StartTime:    time.Now(),
		LastTickTime: time.Now(),
	}
}

func (s *Service[T]) Started() {
	s.State.Started(s.getCtx())
}

func (s *Service[T]) Tick(dt time.Duration) {
	s.State.Tick(s.getCtx(), dt)
}

func (s *Service[T]) Handler() {
	s.State.Handler(s.getCtx())
}

func (s *Service[T]) Stopped() {
	s.State.Stopped(s.getCtx())
}

var _ IService = (*Service[IServiceState])(nil)

func NewBaseNode(name string) *BaseNode {
	return &BaseNode{
		name:     name,
		children: make([]Node, 0),
	}
}

func (n *BaseNode) ID() string {
	return n.name
}

func (n *BaseNode) AddChild(child Node) {
	n.children = append(n.children, child)
}

func (n *BaseNode) RemoveChild(child Node) bool {
	for i, c := range n.children {
		if c.ID() == child.ID() {
			n.children = append(n.children[:i], n.children[i+1:]...)
			return true
		}
	}
	return false
}

func (n *BaseNode) Children() []Node {
	return n.children
}

func (n *BaseNode) ChildCount() int {
	return len(n.children)
}

func (n *BaseNode) SetLocked(locked bool) {
	n.locked = locked
	for _, c := range n.children {
		c.SetLocked(locked)
	}
}

func (n *BaseNode) IsReady() bool {
	if n.locked {
		return false
	}
	for _, child := range n.children {
		if !child.IsReady() {
			return false
		}
	}
	return true
}

func (n *BaseNode) IsLeaf() bool {
	return len(n.children) == 0
}

type CommandAddService struct {
	Service *Service[IServiceState]
}

type CommandFireEvent struct {
	Event Event
}

type CommandEventDone struct {
	Service    *Service[IServiceState]
	WaitUnlock []Node
}

type CommandShutdown struct {
	ok chan struct{}
}

type World struct {
	services      map[string]*Service[IServiceState]
	commands      chan any
	events        *list.List
	stopTimerChan chan struct{}
}

func NewWorld(services ...*Service[IServiceState]) *World {
	svcMap := make(map[string]*Service[IServiceState])
	for _, svc := range services {
		svcMap[svc.ID()] = svc
	}
	return &World{
		services:      svcMap,
		commands:      make(chan any, 1024),
		events:        list.New(),
		stopTimerChan: make(chan struct{}),
	}
}

func (w *World) AddService(service *Service[IServiceState]) {
	w.commands <- CommandAddService{service}
	w.commands <- CommandFireEvent{Event{
		From: "",
		To:   service.ID(),
		Type: EventTypeStarted,
	}}
}

func (w *World) doEvent(s *Service[IServiceState], e Event) {
	group := append(s.Children(), e.Children()...)
	for _, node := range group {
		node.SetLocked(true)
	}
	go func() {
		switch e.Type {
		case EventTypeStarted:
			s.Started()
		case EventTypeTick:
			now := time.Now()
			dt := now.Sub(s.LastTickTime)
			s.LastTickTime = now
			s.Tick(dt)
		case EventTypeClientReq:
			s.Handler()
		}
		w.commands <- CommandEventDone{
			Service:    s,
			WaitUnlock: group,
		}
	}()
}

func (w *World) tick(dt time.Duration) {
	for _, service := range w.services {
		if service.Timer.Tick(dt).JustFinished() {
			w.commands <- CommandFireEvent{
				Event: Event{
					From: "",
					To:   service.ID(),
					Type: EventTypeTick,
				},
			}
		}
	}
}

func (w *World) Start() {
	go func() {
		go func() {
			for id := range w.services {
				w.commands <- CommandFireEvent{Event{
					From: "",
					To:   id,
					Type: EventTypeStarted,
				}}
			}
		}()

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
				w.tick(dt)
			}
		}()

		// mainloop
		for cmd := range w.commands {
			switch c := cmd.(type) {
			case CommandAddService:
				w.services[c.Service.ID()] = c.Service
			case CommandFireEvent:
				if service, exist := w.services[c.Event.To]; exist {
					if service.IsReady() && c.Event.IsReady() {
						w.doEvent(service, c.Event)
					} else {
						w.events.PushBack(c.Event)
					}
				} else {
					log.Printf("Service<%s> does not exist, event %+v discarded", c.Event.To, c.Event)
				}
			case CommandEventDone:
				for _, node := range c.WaitUnlock {
					node.SetLocked(false)
				}
				if c.Service.Exited {
					delete(w.services, c.Service.ID())
					log.Printf("Service<%s> exited", c.Service.ID())
				}
			case CommandShutdown:
				close(w.stopTimerChan)
				close(c.ok)
				return
			}
		}
	}()
}

func (w *World) Stop() {
	ok := make(chan struct{})
	w.commands <- CommandShutdown{ok}
	<-ok
}

type Agent struct {
	ID       string
	NickName string
}

func (a *Agent) Started(ctx ServiceCtx) {
	fmt.Printf("Agent %s started\n", a.ID)
	if ctx.self() == "Agent.1" {
		ctx.exit()
	}
}
func (a *Agent) Stopped(ctx ServiceCtx) {}
func (a *Agent) Tick(ctx ServiceCtx, dt time.Duration) {
	fmt.Printf("Agent %s tick, dt: %+v\n", a.ID, dt)
}
func (a *Agent) Handler(ctx ServiceCtx) {}

func main() {
	world := NewWorld(
		NewService("Agent.1", time.Second, &Agent{ID: "1", NickName: "Jack"}),
		NewService("Agent.2", time.Second, &Agent{ID: "2", NickName: "Lily"}),
	)
	world.Start()
	time.Sleep(time.Second * 3)
	world.Stop()
}
