// src/types/types.go - 共享类型定义（静态库）
package types

import (
	"encoding/json"
	"fmt"
)

// ==================== 位置信息 ====================

type Position struct {
	File   string
	Line   int
	Column int
}

func (p Position) String() string {
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
}

// ==================== AST节点定义 ====================

type Node interface {
	Pos() Position
}

type Program struct {
	Roles    []*Role
	FilePath string
	Position Position
}

func (p *Program) Pos() Position { return p.Position }

type Role struct {
	Name      string
	Events    []*Event
	Variables map[string]interface{}
	Position  Position
}

func (r *Role) Pos() Position { return r.Position }

type Event struct {
	Type     string
	Message  string
	Actions  []map[string]interface{}
	Special  string
	Position Position
}

func (e *Event) Pos() Position { return e.Position }

// ==================== 辅助函数 ====================

func LoadProgramFromJSON(data []byte) (*Program, error) {
	var program Program
	err := json.Unmarshal(data, &program)
	if err != nil {
		return nil, err
	}
	return &program, nil
}

func SaveProgramToJSON(program *Program) ([]byte, error) {
	return json.MarshalIndent(program, "", "  ")
}

func NewProgram() *Program {
	return &Program{
		Roles:    make([]*Role, 0),
		Position: Position{},
	}
}

func NewRole(name string) *Role {
	return &Role{
		Name:      name,
		Events:    make([]*Event, 0),
		Variables: make(map[string]interface{}),
		Position:  Position{},
	}
}

func NewEvent(eventType string) *Event {
	return &Event{
		Type:     eventType,
		Actions:  make([]map[string]interface{}, 0),
		Position: Position{},
	}
}

func (p *Program) AddRole(role *Role) {
	p.Roles = append(p.Roles, role)
}

func (r *Role) AddEvent(event *Event) {
	r.Events = append(r.Events, event)
}

func (e *Event) AddAction(action map[string]interface{}) {
	e.Actions = append(e.Actions, action)
}

func (r *Role) SetVariable(name string, value interface{}) {
	r.Variables[name] = value
}

func (r *Role) GetVariable(name string) interface{} {
	return r.Variables[name]
}