package server

import "github.com/julywind168/flying"

type Session struct {
	flying.BaseNode
}

func (s *Session) Response(result any) {
}

func (s *Session) Push(name string, params any) {
}

var _ flying.ISession = (*Session)(nil)
