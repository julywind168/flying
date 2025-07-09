package flying

import (
	"log"
	"reflect"
	"time"
)

type UService interface {
	Started(ctx ServiceCtx)
	Tick(ctx ServiceCtx, dt time.Duration)
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

type Message struct {
	Name   string
	Params any
}

type ISession interface {
	Node
	Response(result any)
	Push(name string, params any)
}

type Service[T UService] struct {
	BaseNode
	World  *World
	Timer  Timer
	State  T
	Exited bool
}
type ServiceCtx interface {
	Self() string
	Exit()
	Send(to string, name string, params any)
	FireClientRequest(to string, session ISession, name string, params any)
}

type internalServiceCtx struct {
	selfFunc              func() string
	exitFunc              func()
	sendFunc              func(to string, name string, params any)
	fireClientRequestFunc func(to string, session ISession, name string, params any)
}

func (c *internalServiceCtx) Self() string {
	return c.selfFunc()
}

func (c *internalServiceCtx) Exit() {
	c.exitFunc()
}

func (c *internalServiceCtx) Send(to string, name string, params any) {
	c.sendFunc(to, name, params)
}

func (c *internalServiceCtx) FireClientRequest(to string, session ISession, name string, params any) {
	c.fireClientRequestFunc(to, session, name, params)
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
		fireClientRequestFunc: func(to string, session ISession, name string, params any) {
			s.World.FireClientRequest(to, session, name, params)
		},
	}
}

func (s *Service[T]) send(to string, name string, params any) {
	s.World.FireServiceMessage(s.ID(), to, name, params)
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

	if method.Type().NumIn() != 3 ||
		!reflect.TypeOf(ctx).AssignableTo(method.Type().In(0)) ||
		!reflect.TypeOf(from).AssignableTo(method.Type().In(1)) ||
		!reflect.TypeOf(msg.Params).AssignableTo(method.Type().In(2)) {
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

	if method.Type().NumIn() != 3 ||
		!reflect.TypeOf(ctx).AssignableTo(method.Type().In(0)) ||
		!reflect.TypeOf(session).AssignableTo(method.Type().In(1)) ||
		!reflect.TypeOf(msg.Params).AssignableTo(method.Type().In(2)) {
		log.Printf("Service %s method %s does not have the correct signature", s.ID(), msg.Name)
		return
	}

	method.Call(args)
}

func (s *Service[T]) Stopped() {
	s.State.Stopped(s.getCtx())
}

var _ IService = (*Service[UService])(nil)
