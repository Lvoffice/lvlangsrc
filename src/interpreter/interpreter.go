// src/interpreter/main.go - 解释器 DLL
package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"LVLANG/src/types"
)

// ==================== 运行时状态 ====================

type RoleState struct {
	Variables map[string]interface{}
	mu        sync.RWMutex
}

type Interpreter struct {
	roles     map[string]*RoleState
	program   *types.Program
	debug     bool
	wg        sync.WaitGroup
	mu        sync.RWMutex
	errors    *RuntimeErrorCollector
	startTime time.Time
	filename  string
}

// ==================== 运行时错误收集器 ====================

type RuntimeErrorLevel int

const (
	RuntimeLevelInfo RuntimeErrorLevel = iota
	RuntimeLevelWarning
	RuntimeLevelError
	RuntimeLevelFatal
)

type RuntimeError struct {
	Level   RuntimeErrorLevel
	Role    string
	Event   string
	Message string
}

type RuntimeErrorCollector struct {
	errors   []RuntimeError
	debug    bool
	warnings int
}

func NewRuntimeErrorCollector(debug bool) *RuntimeErrorCollector {
	return &RuntimeErrorCollector{
		errors: make([]RuntimeError, 0),
		debug:  debug,
	}
}

func (rec *RuntimeErrorCollector) AddError(role, event, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	rec.errors = append(rec.errors, RuntimeError{
		Level:   RuntimeLevelError,
		Role:    role,
		Event:   event,
		Message: msg,
	})
}

func (rec *RuntimeErrorCollector) AddWarning(role, event, format string, args ...interface{}) {
	if !rec.debug {
		rec.warnings++
		return
	}
	msg := fmt.Sprintf(format, args...)
	rec.errors = append(rec.errors, RuntimeError{
		Level:   RuntimeLevelWarning,
		Role:    role,
		Event:   event,
		Message: msg,
	})
}

func (rec *RuntimeErrorCollector) AddInfo(role, event, format string, args ...interface{}) {
	if !rec.debug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	rec.errors = append(rec.errors, RuntimeError{
		Level:   RuntimeLevelInfo,
		Role:    role,
		Event:   event,
		Message: msg,
	})
}

func (rec *RuntimeErrorCollector) HasErrors() bool {
	for _, err := range rec.errors {
		if err.Level >= RuntimeLevelError {
			return true
		}
	}
	return false
}

func (rec *RuntimeErrorCollector) Report() string {
	if len(rec.errors) == 0 && rec.warnings == 0 {
		return ""
	}

	var sb strings.Builder

	if rec.debug {
		for _, err := range rec.errors {
			switch err.Level {
			case RuntimeLevelError:
				sb.WriteString(fmt.Sprintf("运行时错误 [%s.%s]: %s\n", err.Role, err.Event, err.Message))
			case RuntimeLevelWarning:
				sb.WriteString(fmt.Sprintf("运行时警告 [%s.%s]: %s\n", err.Role, err.Event, err.Message))
			case RuntimeLevelInfo:
				sb.WriteString(fmt.Sprintf("运行时信息 [%s.%s]: %s\n", err.Role, err.Event, err.Message))
			}
		}
	} else {
		errorCount := 0
		warningCount := rec.warnings
		for _, err := range rec.errors {
			if err.Level >= RuntimeLevelError {
				errorCount++
			}
		}
		if errorCount > 0 {
			sb.WriteString(fmt.Sprintf("运行时发现 %d 个错误", errorCount))
			if warningCount > 0 {
				sb.WriteString(fmt.Sprintf("，%d 个警告", warningCount))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ==================== 解释器核心 ====================

func NewInterpreter(debug bool, filename string) *Interpreter {
	return &Interpreter{
		roles:     make(map[string]*RoleState),
		debug:     debug,
		errors:    NewRuntimeErrorCollector(debug),
		startTime: time.Now(),
		filename:  filename,
	}
}

func (i *Interpreter) GetErrors() *RuntimeErrorCollector {
	return i.errors
}

func (i *Interpreter) log(format string, args ...interface{}) {
	if i.debug {
		fmt.Printf("[解释器] "+format+"\n", args...)
	}
}

func (i *Interpreter) LoadProgramFromCache() bool {
	i.log("尝试从缓存加载: %s", i.filename)

	// 使用与 Parser 相同的缓存键生成逻辑
	absPath, err := filepath.Abs(i.filename)
	if err != nil {
		absPath = i.filename
	}
	key := strings.ReplaceAll(absPath, string(filepath.Separator), "_")
	key = strings.ReplaceAll(key, ":", "_")
	cacheFile := "cache/" + key + ".json"

	data, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		i.log("缓存文件不存在: %s", cacheFile)
		return false
	}

	program, err := types.LoadProgramFromJSON(data)
	if err != nil {
		i.log("解析 JSON 失败: %v", err)
		return false
	}

	i.program = program
	i.log("成功从缓存加载程序")

	for _, role := range program.Roles {
		state := &RoleState{
			Variables: make(map[string]interface{}),
		}
		for k, v := range role.Variables {
			state.Variables[k] = v
		}
		i.roles[role.Name] = state
		i.log("创建角色: %s", role.Name)
	}

	return true
}

func (i *Interpreter) SetProgram(program *types.Program) {
	i.program = program

	for _, role := range program.Roles {
		state := &RoleState{
			Variables: make(map[string]interface{}),
		}
		for k, v := range role.Variables {
			state.Variables[k] = v
		}
		i.roles[role.Name] = state
		i.log("创建角色: %s", role.Name)
	}
}

func (i *Interpreter) Execute() {
	if i.program == nil {
		i.errors.AddError("系统", "启动", "没有加载程序")
		return
	}

	i.log("开始执行，共 %d 个角色", len(i.program.Roles))

	for _, role := range i.program.Roles {
		for _, event := range role.Events {
			if event.Type == "开始" || event.Type == "start" {
				i.executeEvent(role.Name, event)
			}
		}
	}

	i.wg.Wait()
	i.log("执行完成，耗时: %v", time.Since(i.startTime))
}

func (i *Interpreter) executeEvent(roleName string, event *types.Event) {
	i.wg.Add(1)
	go func(roleName string, event *types.Event) {
		defer i.wg.Done()

		prefix := ""
		if event.Special != "" {
			prefix = event.Special
		}
		i.log("%s[%s] 执行事件: %s", prefix, roleName, event.Type)

		state := i.roles[roleName]

		for _, action := range event.Actions {
			i.executeAction(roleName, state, action)
		}
	}(roleName, event)
}

func (i *Interpreter) executeAction(roleName string, state *RoleState, action map[string]interface{}) {
	actionType, _ := action["type"].(string)

	switch actionType {
	case "说":
		content, _ := action["content"].(string)
		fmt.Println(content)
		i.log("[%s] 说: %s", roleName, content)

	case "移动":
		steps := i.resolveValue(state, action["steps"])
		if stepsNum, ok := steps.(float64); ok {
			state.mu.Lock()
			if val, ok := state.Variables["x"]; ok {
				if x, ok := val.(float64); ok {
					state.Variables["x"] = x + stepsNum
				}
			} else {
				state.Variables["x"] = stepsNum
			}
			state.mu.Unlock()
			i.log("[%s] 移动: %v步", roleName, stepsNum)
		}

	case "旋转":
		angle := i.resolveValue(state, action["angle"])
		if angleNum, ok := angle.(float64); ok {
			state.mu.Lock()
			if val, ok := state.Variables["方向"]; ok {
				if dir, ok := val.(float64); ok {
					state.Variables["方向"] = dir + angleNum
				}
			} else {
				state.Variables["方向"] = angleNum
			}
			state.mu.Unlock()
			i.log("[%s] 旋转: %v度", roleName, angleNum)
		}

	case "等待":
		seconds := i.resolveValue(state, action["seconds"])
		if secNum, ok := seconds.(float64); ok {
			i.log("[%s] 等待: %v秒", roleName, secNum)
			time.Sleep(time.Duration(secNum * float64(time.Second)))
		}

	case "广播":
		msg, _ := action["message"].(string)
		i.log("[%s] 广播: %s", roleName, msg)
		i.broadcast(roleName, msg)

	case "赋值":
		name, _ := action["variable"].(string)
		value := i.resolveValue(state, action["value"])
		state.mu.Lock()
		state.Variables[name] = value
		state.mu.Unlock()
		i.log("[%s] 赋值: %s = %v", roleName, name, value)

	default:
		i.errors.AddWarning(roleName, "未知", "未知动作类型: %s", actionType)
	}
}

func (i *Interpreter) resolveValue(state *RoleState, val interface{}) interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		if v["type"] == "variable" {
			name, _ := v["name"].(string)
			state.mu.RLock()
			defer state.mu.RUnlock()
			if result, ok := state.Variables[name]; ok {
				return result
			}
			return nil
		}
		return v
	default:
		return v
	}
}

func (i *Interpreter) broadcast(sender string, message string) {
	for _, role := range i.program.Roles {
		for _, event := range role.Events {
			if event.Type == "收到" || event.Type == "message" {
				if event.Message == message {
					i.log("触发 %s 的收到事件: %s", role.Name, message)
					i.executeEvent(role.Name, event)
				}
			}
		}
	}
}

// ==================== DLL 导出函数 ====================

//export ExecuteFile
func ExecuteFile(filename *C.char) *C.char {
	goFilename := C.GoString(filename)

	interp := NewInterpreter(false, goFilename)
	if !interp.LoadProgramFromCache() {
		return C.CString("加载缓存失败")
	}

	interp.Execute()

	if interp.GetErrors().HasErrors() {
		return C.CString("执行失败")
	}

	return C.CString("执行完成")
}

//export ExecuteFileDebug
func ExecuteFileDebug(filename *C.char) *C.char {
	goFilename := C.GoString(filename)

	interp := NewInterpreter(true, goFilename)
	if !interp.LoadProgramFromCache() {
		return C.CString("加载缓存失败")
	}

	interp.Execute()

	if interp.GetErrors().HasErrors() {
		return C.CString("调试执行失败")
	}

	return C.CString("调试执行完成")
}

//export FreeString
func FreeString(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}

func main() {}
