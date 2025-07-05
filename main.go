package main

import (
	"container/list"
	"fmt"
	"log"
	"reflect"
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
	Tick()
	Message(from string, msg Message)
	Handler(session ISession, msg Message)
	Stopped()
}

type EventType uint8

const (
	EventTypeStarted EventType = iota
	EventTypeTick
	EventTypeMessage // message from other service
	EventTypeClientReq
)

type Message struct {
	Name   string
	Params any
}

type ISession interface {
	Node
	Response(result any)
	Push(name string, params any)
}

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
	id       string
	locked   bool
	children []Node
}

type Service[T IServiceState] struct {
	BaseNode
	World  *World
	Timer  Timer
	State  T
	Exited bool
}

type ServiceCtx interface {
	self() string
	exit()
	send(to string, name string, params any)
}

type internalServiceCtx struct {
	selfFunc func() string
	exitFunc func()
	sendFunc func(to string, name string, params any)
}

func (c *internalServiceCtx) self() string {
	return c.selfFunc()
}

func (c *internalServiceCtx) exit() {
	c.exitFunc()
}

func (c *internalServiceCtx) send(to string, name string, params any) {
	c.sendFunc(to, name, params)
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
		sendFunc: func(to string, name string, params any) {
			s.send(to, name, params)
		},
	}
}

func (s *Service[T]) send(to string, name string, params any) {
	s.World.commands <- CommandFireEvent{
		Event: Event{
			From: s.ID(),
			To:   to,
			Type: EventTypeMessage,
			Playload: Message{
				Name:   name,
				Params: params,
			},
		},
	}
}

func (s *Service[T]) Started() {
	s.State.Started(s.getCtx())
}

func (s *Service[T]) Tick() {
	s.State.Tick(s.getCtx(), s.Timer.Interval)
}

func (s *Service[T]) Message(from string, msg Message) {
	v := reflect.ValueOf(s.State)
	method := v.MethodByName(msg.Name)
	if !method.IsValid() {
		log.Printf("Service %s does not have a method called %s", s.ID(), msg.Name)
		return
	}

	ctx := s.getCtx()

	args := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(from),
		reflect.ValueOf(msg.Params),
	}

	if method.Type().NumIn() != 3 || !reflect.TypeOf(msg.Params).AssignableTo(method.Type().In(2)) {
		log.Printf("Service %s method %s does not have the correct signature", s.ID(), msg.Name)
		return
	}

	method.Call(args)
}

func (s *Service[T]) Handler(session ISession, msg Message) {
	v := reflect.ValueOf(s.State)
	method := v.MethodByName(msg.Name)
	if !method.IsValid() {
		log.Printf("Service %s does not have a method called %s", s.ID(), msg.Name)
		return
	}

	ctx := s.getCtx()

	args := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(session),
		reflect.ValueOf(msg.Params),
	}

	if method.Type().NumIn() != 3 || !reflect.TypeOf(msg.Params).AssignableTo(method.Type().In(2)) {
		log.Printf("Service %s method %s does not have the correct signature", s.ID(), msg.Name)
		return
	}

	method.Call(args)
}

func (s *Service[T]) Stopped() {
	s.State.Stopped(s.getCtx())
}

var _ IService = (*Service[IServiceState])(nil)

func NewBaseNode(id string) *BaseNode {
	return &BaseNode{
		id:       id,
		children: make([]Node, 0),
	}
}

func (n *BaseNode) ID() string {
	return n.id
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

type CommandTick struct {
	Delta time.Duration
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

func NewWorld() *World {
	return &World{
		services:      make(map[string]*Service[IServiceState]),
		commands:      make(chan any, 1024),
		events:        list.New(),
		stopTimerChan: make(chan struct{}),
	}
}

func Spawn[T IServiceState](w *World, id string, tickInterval time.Duration, state T) {
	if tickInterval < SERVICE_MIN_TICK_DURATION {
		log.Panicf("Service tick interval cannot be less than %v", SERVICE_MIN_TICK_DURATION)
	}
	service := &Service[IServiceState]{
		BaseNode: *NewBaseNode(id),
		World:    w,
		Timer: Timer{
			Interval: tickInterval,
			Elapsed:  0,
		},
		State: state,
	}

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
			s.Tick()
		case EventTypeMessage:
			s.Message(e.From, e.Playload.(Message))
		case EventTypeClientReq:
			session := e.Children()[0].(ISession)
			s.Handler(session, e.Playload.(Message))
		}
		w.commands <- CommandEventDone{
			Service:    s,
			WaitUnlock: group,
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

func (w *World) PushClientRequest(to string, session IService, msg Message) {
	w.commands <- CommandFireEvent{
		Event{
			BaseNode: BaseNode{
				children: []Node{session},
			},
			From: "",
			To:   to,
			Type: EventTypeClientReq,
			Playload: Message{
				Name:   msg.Name,
				Params: msg.Params,
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
				w.commands <- CommandTick{Delta: dt}
			}
		}()

		// mainloop
		for cmd := range w.commands {
			switch c := cmd.(type) {
			case CommandAddService:
				w.services[c.Service.ID()] = c.Service
			case CommandTick:
				w.tick(c.Delta)
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
				w.tryDispatchPendingEvents()
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
}
func (a *Agent) Stopped(ctx ServiceCtx) {}
func (a *Agent) Tick(ctx ServiceCtx, dt time.Duration) {
	fmt.Printf("Agent %s tick, dt: %+v\n", a.ID, dt)
	if a.ID == "2" {
		ctx.send("Agent.1", "Ping", PingPayload{Msg: "hello world"})
	}
}
func (a *Agent) Handler(ctx ServiceCtx) {}

type PingPayload struct {
	Msg string
}

func (a *Agent) Ping(ctx ServiceCtx, from string, payload PingPayload) {
	fmt.Printf("Agent %s ping from %s, payload: %+v\n", a.ID, from, payload.Msg)
}

func main() {
	world := NewWorld()
	Spawn(world, "Agent.1", time.Second, &Agent{ID: "1", NickName: "Jack"})
	Spawn(world, "Agent.2", time.Second, &Agent{ID: "2", NickName: "Lily"})
	world.Start()
	time.Sleep(time.Second * 3)
	world.Stop()
}
