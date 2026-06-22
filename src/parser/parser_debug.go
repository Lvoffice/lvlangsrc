package main

import (
	"fmt"

	"LVLANG/src/types"
)

type DebugParser struct {
	*Parser
}

func NewDebugParser(source, filename string, grammar map[string][]string) *DebugParser {
	return &DebugParser{
		Parser: NewParser(source, filename, grammar, true),
	}
}

func (p *DebugParser) log(format string, args ...interface{}) {
	fmt.Printf("[解析器调试] "+format+"\n", args...)
}

func (p *DebugParser) Parse() *types.Program {
	p.log("开始解析文件: %s", p.filename)
	p.log("源代码长度: %d 字符", len(p.source))
	if len(p.source) > 50 {
		p.log("前50字符: %q", p.source[:50])
	}
	p.advance()
	program := types.NewProgram()
	program.FilePath = p.filename
	program.Position = types.Position{File: p.filename, Line: 1, Column: 1}
	tokenCount := 0
	for p.current.Type != TOKEN_EOF {
		tokenCount++
		p.log("Token %d: 类型=%d, 值=%q, 位置=%d:%d",
			tokenCount, p.current.Type, p.current.Value, p.current.Line, p.current.Column)
		if p.matchKeyword("角色") {
			p.log("发现角色定义")
			role := p.parseRole()
			if role != nil {
				program.AddRole(role)
			}
		} else {
			p.advance()
		}
	}
	p.log("解析完成，共 %d 个 token，发现 %d 个角色", tokenCount, len(program.Roles))
	return program
}

func (p *DebugParser) parseRole() *types.Role {
	p.log("开始解析角色")
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	name, namePos := p.parseIdentifier()
	if name == "" {
		p.log("错误: 角色名不能为空")
		p.errors.AddError(namePos, "角色名不能为空")
		return nil
	}
	p.log("角色名: %s", name)
	role := types.NewRole(name)
	role.Position = pos
	if !p.expectSymbol("{") {
		p.log("错误: 缺少 '{'")
		return nil
	}
	p.log("成功匹配 '{'")
	for p.current.Type != TOKEN_EOF && !p.matchSymbol("}") {
		p.log("当前令牌: 类型=%d, 值=%q", p.current.Type, p.current.Value)
		if p.current.Type == TOKEN_SYMBOL && p.current.Value == "@" {
			p.log("发现特殊事件标记 @")
			p.advance()
			if p.matchKeyword("事件") {
				event := p.parseEvent()
				if event != nil {
					event.Special = "@"
					role.AddEvent(event)
				}
			}
			continue
		}
		if p.matchKeyword("事件") {
			p.log("发现事件")
			event := p.parseEvent()
			if event != nil {
				role.AddEvent(event)
			}
			continue
		}
		if p.matchKeyword("变量") {
			p.log("发现变量声明")
			varDecl := p.parseVariableDecl()
			if varDecl != nil {
				role.SetVariable(varDecl["name"].(string), varDecl["value"])
			}
			continue
		}
		p.log("跳过未知内容")
		p.advance()
	}
	p.log("角色解析完成，发现 %d 个事件", len(role.Events))
	return role
}

func (p *DebugParser) parseEvent() *types.Event {
	p.log("开始解析事件")
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	typeToken := p.current
	if typeToken.Type != TOKEN_IDENTIFIER {
		p.log("错误: 事件需要类型")
		p.errors.AddError(pos, "事件需要类型")
		return nil
	}
	p.advance()
	p.log("事件类型: %s", typeToken.Value)
	event := types.NewEvent(typeToken.Value)
	event.Position = pos
	if event.Type == "收到" || event.Type == "message" {
		if p.current.Type == TOKEN_STRING {
			msg, _ := p.parseString()
			event.Message = msg
			p.log("收到事件消息: %s", msg)
		} else {
			p.log("错误: 收到事件需要消息内容")
			p.errors.AddError(pos, "收到事件需要消息内容")
		}
	}
	if !p.expectSymbol("{") {
		p.log("错误: 缺少 '{'")
		return nil
	}
	p.log("开始解析动作块")
	actionCount := 0
	for p.current.Type != TOKEN_EOF && !p.matchSymbol("}") {
		action := p.parseAction()
		if action != nil {
			actionCount++
			event.AddAction(action)
		}
	}
	p.log("动作块解析完成，发现 %d 个动作", actionCount)
	return event
}

func (p *DebugParser) parseAction() map[string]interface{} {
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	if p.matchKeyword("说") {
		p.log("解析说动作")
		content, contentPos := p.parseString()
		if content == "" {
			p.log("错误: 说动作需要内容")
			p.errors.AddError(contentPos, "说动作需要内容")
			return nil
		}
		p.matchSymbol(";")
		p.log("说内容: %s", content)
		return map[string]interface{}{"type": "说", "content": content}
	}
	if p.matchKeyword("移动") {
		p.log("解析移动动作")
		steps := p.parseValue()
		if steps == nil {
			p.log("错误: 移动动作需要步数")
			p.errors.AddError(pos, "移动动作需要步数")
			return nil
		}
		p.matchKeyword("步")
		p.matchSymbol(";")
		p.log("移动步数: %v", steps)
		return map[string]interface{}{"type": "移动", "steps": steps}
	}
	if p.matchKeyword("旋转") {
		p.log("解析旋转动作")
		angle := p.parseValue()
		if angle == nil {
			p.log("错误: 旋转动作需要角度")
			p.errors.AddError(pos, "旋转动作需要角度")
			return nil
		}
		p.matchKeyword("度")
		p.matchSymbol(";")
		p.log("旋转角度: %v", angle)
		return map[string]interface{}{"type": "旋转", "angle": angle}
	}
	if p.matchKeyword("等待") {
		p.log("解析等待动作")
		seconds := p.parseValue()
		if seconds == nil {
			p.log("错误: 等待动作需要秒数")
			p.errors.AddError(pos, "等待动作需要秒数")
			return nil
		}
		p.matchKeyword("秒")
		p.matchSymbol(";")
		p.log("等待秒数: %v", seconds)
		return map[string]interface{}{"type": "等待", "seconds": seconds}
	}
	if p.matchKeyword("广播") {
		p.log("解析广播动作")
		msg, msgPos := p.parseString()
		if msg == "" {
			p.log("错误: 广播动作需要消息")
			p.errors.AddError(msgPos, "广播动作需要消息")
			return nil
		}
		p.matchSymbol(";")
		p.log("广播消息: %s", msg)
		return map[string]interface{}{"type": "广播", "message": msg}
	}
	if p.current.Type == TOKEN_IDENTIFIER {
		name, namePos := p.parseIdentifier()
		if p.matchSymbol("=") {
			p.log("解析赋值语句: %s = ?", name)
			value := p.parseValue()
			if value == nil {
				p.log("错误: 赋值语句需要值")
				p.errors.AddError(namePos, "赋值语句需要值")
				return nil
			}
			p.matchSymbol(";")
			p.log("赋值: %s = %v", name, value)
			return map[string]interface{}{"type": "赋值", "variable": name, "value": value}
		}
	}
	p.log("无法识别的动作")
	return nil
}
