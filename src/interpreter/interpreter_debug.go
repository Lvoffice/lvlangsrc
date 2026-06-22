// src/interpreter/interpreter_debug.go - 调试版解释器
package main

import (
	"fmt"
	"time"

	"LVLANG/src/types"
)

type DebugInterpreter struct {
	*Interpreter
}

func NewDebugInterpreter(filename string) *DebugInterpreter {
	return &DebugInterpreter{
		Interpreter: NewInterpreter(true, filename),
	}
}

func (i *DebugInterpreter) log(format string, args ...interface{}) {
	fmt.Printf("[解释器调试] "+format+"\n", args...)
}

func (i *DebugInterpreter) Execute() {
	if i.program == nil {
		i.errors.AddError("系统", "启动", "没有加载程序")
		return
	}

	i.log("开始执行，共 %d 个角色", len(i.program.Roles))
	i.log("角色列表:")
	for _, role := range i.program.Roles {
		i.log("  - %s (变量: %d, 事件: %d)",
			role.Name, len(role.Variables), len(role.Events))
	}

	startTime := time.Now()

	eventCount := 0
	for _, role := range i.program.Roles {
		for _, event := range role.Events {
			if event.Type == "开始" || event.Type == "start" {
				eventCount++
				i.log("触发事件: %s.%s", role.Name, event.Type)
				i.executeEvent(role.Name, event)
			}
		}
	}
	i.log("共触发 %d 个开始事件", eventCount)

	i.wg.Wait()

	elapsed := time.Since(startTime)
	i.log("执行完成，耗时: %v", elapsed)
}

func (i *DebugInterpreter) executeEvent(roleName string, event *types.Event) {
	i.wg.Add(1)
	go func(roleName string, event *types.Event) {
		defer i.wg.Done()

		prefix := ""
		if event.Special != "" {
			prefix = event.Special
		}
		i.log("%s[%s] 执行事件: %s (消息: %s, 动作数: %d)",
			prefix, roleName, event.Type, event.Message, len(event.Actions))

		state := i.roles[roleName]

		for idx, action := range event.Actions {
			i.log("  执行动作 %d: %v", idx, action["type"])
			i.executeAction(roleName, state, action)
		}
	}(roleName, event)
}

func (i *DebugInterpreter) executeAction(roleName string, state *RoleState, action map[string]interface{}) {
	actionType, _ := action["type"].(string)

	i.log("    当前变量: %v", state.Variables)

	switch actionType {
	case "说":
		content, _ := action["content"].(string)
		i.log("    说内容: %s", content)
		fmt.Println(content)

	case "移动":
		steps := i.resolveValue(state, action["steps"])
		i.log("    移动步数: %v", steps)
		if stepsNum, ok := steps.(float64); ok {
			state.mu.Lock()
			oldX, _ := state.Variables["x"].(float64)
			state.Variables["x"] = oldX + stepsNum
			state.mu.Unlock()
			i.log("    x: %v -> %v", oldX, oldX+stepsNum)
		}

	case "旋转":
		angle := i.resolveValue(state, action["angle"])
		i.log("    旋转角度: %v", angle)
		if angleNum, ok := angle.(float64); ok {
			state.mu.Lock()
			oldDir, _ := state.Variables["方向"].(float64)
			state.Variables["方向"] = oldDir + angleNum
			state.mu.Unlock()
			i.log("    方向: %v -> %v", oldDir, oldDir+angleNum)
		}

	case "等待":
		seconds := i.resolveValue(state, action["seconds"])
		i.log("    等待: %v秒", seconds)
		if secNum, ok := seconds.(float64); ok {
			time.Sleep(time.Duration(secNum * float64(time.Second)))
		}

	case "广播":
		msg, _ := action["message"].(string)
		i.log("    广播消息: %s", msg)
		i.broadcast(roleName, msg)

	case "赋值":
		name, _ := action["variable"].(string)
		value := i.resolveValue(state, action["value"])
		state.mu.Lock()
		oldVal := state.Variables[name]
		state.Variables[name] = value
		state.mu.Unlock()
		i.log("    赋值: %s = %v (原值: %v)", name, value, oldVal)

	default:
		i.errors.AddWarning(roleName, "未知", "未知动作类型: %s", actionType)
	}
}

func (i *DebugInterpreter) broadcast(sender string, message string) {
	i.log("广播消息 '%s' 来自 %s", message, sender)

	count := 0
	for _, role := range i.program.Roles {
		for _, event := range role.Events {
			if event.Type == "收到" || event.Type == "message" {
				if event.Message == message {
					count++
					i.log("  触发 %s 的收到事件", role.Name)
					i.executeEvent(role.Name, event)
				}
			}
		}
	}
	i.log("共触发 %d 个收到事件", count)
}