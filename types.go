package flying

type Node interface {
	ID() string
	AddChild(child Node)
	RemoveChild(child Node) bool
	ChildCount() int
	Children() []Node
	SetLocked(bool)
	IsReady() bool
}

type EventType uint8

const (
	EventTypeStarted EventType = iota
	EventTypeTick
	EventTypeMessage // message from other service
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
	id       string
	locked   bool
	children []Node
}

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
