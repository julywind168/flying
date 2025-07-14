package flying

import (
	"encoding/json"
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
	Handler(session ISession, req Request)
	Stopped()
}

type Message struct {
	Name   string
	Params any
}

type Request struct {
	Name   string
	Params []byte
}

type ISession interface {
	Node
	Response(result any)
	Push(name string, payload any)
}

type Service[T UService] struct {
	BaseNode
	World  *World
	Timer  Timer
	State  T
	Exited bool
}
type ServiceCtx interface {
	Spawn(id string, tickInterval time.Duration, state UService)
	Self() string
	Exit()
	Send(to string, name string, params any)
	FireClientRequest(to string, session ISession, name string, params []byte)
}

type internalServiceCtx struct {
	spawnFunc             func(id string, tickInterval time.Duration, state UService)
	selfFunc              func() string
	exitFunc              func()
	sendFunc              func(to string, name string, params any)
	fireClientRequestFunc func(to string, session ISession, name string, params []byte)
}

func (c *internalServiceCtx) Spawn(id string, tickInterval time.Duration, state UService) {
	c.spawnFunc(id, tickInterval, state)
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

func (c *internalServiceCtx) FireClientRequest(to string, session ISession, name string, params []byte) {
	c.fireClientRequestFunc(to, session, name, params)
}

func (s *Service[T]) getCtx() ServiceCtx {
	return &internalServiceCtx{
		spawnFunc: s.World.Spawn,
		selfFunc:  s.ID,
		exitFunc: func() {
			if !s.Exited {
				s.Exited = true
				s.Stopped()
			}
		},
		sendFunc: func(to string, name string, params any) {
			s.send(to, name, params)
		},
		fireClientRequestFunc: s.World.FireClientRequest,
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

func (s *Service[T]) Handler(session ISession, req Request) {
	v := reflect.ValueOf(s.State)
	method := v.MethodByName(req.Name)
	if !method.IsValid() {
		log.Printf("Service %s does not have a method called %s", s.ID(), req.Name)
		return
	}

	ctx := s.getCtx()

	methodType := method.Type()
	if methodType.NumIn() != 3 {
		log.Printf("Service %s method %s does not have the correct signature", s.ID(), req.Name)
		return
	}

	paramType := methodType.In(2)
	var paramValue reflect.Value

	if paramType == reflect.TypeOf([]byte(nil)) { // accept raw bytes
		paramValue = reflect.ValueOf(req.Params)
	} else if paramType.Kind() == reflect.Interface && paramType.NumMethod() == 0 { // accept any
		paramValue = reflect.ValueOf(req.Params)
	} else {
		// Try to unmarshal JSON into the expected type from method signature
		paramPtr := reflect.New(paramType)
		if err := json.Unmarshal(req.Params, paramPtr.Interface()); err != nil {
			log.Printf("Service %s method %s: failed to unmarshal params: %v", s.ID(), req.Name, err)
			return
		}
		paramValue = paramPtr.Elem()
	}

	args := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(session),
		paramValue,
	}

	if !reflect.TypeOf(ctx).AssignableTo(methodType.In(0)) ||
		!reflect.TypeOf(session).AssignableTo(methodType.In(1)) {
		log.Printf("Service %s method %s does not have the correct signature", s.ID(), req.Name)
		return
	}

	method.Call(args)
}

func (s *Service[T]) Stopped() {
	s.State.Stopped(s.getCtx())
}

var _ IService = (*Service[UService])(nil)
