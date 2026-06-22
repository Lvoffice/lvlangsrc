package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"LVLANG/src/types"
)

type TokenType int

const (
	TOKEN_EOF TokenType = iota
	TOKEN_IDENTIFIER
	TOKEN_NUMBER
	TOKEN_STRING
	TOKEN_KEYWORD
	TOKEN_SYMBOL
)

type Token struct {
	Type     TokenType
	Value    string
	Category string
	Line     int
	Column   int
}

type ErrorLevel int

const (
	LevelInfo ErrorLevel = iota
	LevelWarning
	LevelError
	LevelFatal
)

type ParseError struct {
	Level   ErrorLevel
	Pos     types.Position
	Message string
	Context string
}

type ErrorCollector struct {
	errors   []ParseError
	debug    bool
	warnings int
	infos    int
}

func NewErrorCollector(debug bool) *ErrorCollector {
	return &ErrorCollector{
		errors: make([]ParseError, 0),
		debug:  debug,
	}
}

func (ec *ErrorCollector) AddError(pos types.Position, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	ec.errors = append(ec.errors, ParseError{
		Level:   LevelError,
		Pos:     pos,
		Message: msg,
	})
}

func (ec *ErrorCollector) AddWarning(pos types.Position, format string, args ...interface{}) {
	if !ec.debug {
		ec.warnings++
		return
	}
	msg := fmt.Sprintf(format, args...)
	ec.errors = append(ec.errors, ParseError{
		Level:   LevelWarning,
		Pos:     pos,
		Message: msg,
	})
}

func (ec *ErrorCollector) HasErrors() bool {
	for _, err := range ec.errors {
		if err.Level >= LevelError {
			return true
		}
	}
	return false
}

func (ec *ErrorCollector) Report() string {
	if len(ec.errors) == 0 && ec.warnings == 0 {
		return ""
	}
	var sb strings.Builder
	if ec.debug {
		for _, err := range ec.errors {
			switch err.Level {
			case LevelError:
				sb.WriteString(fmt.Sprintf("错误 %s: %s\n", err.Pos, err.Message))
			case LevelWarning:
				sb.WriteString(fmt.Sprintf("警告 %s: %s\n", err.Pos, err.Message))
			case LevelInfo:
				sb.WriteString(fmt.Sprintf("信息 %s: %s\n", err.Pos, err.Message))
			}
		}
	} else {
		errorCount := 0
		for _, err := range ec.errors {
			if err.Level >= LevelError {
				errorCount++
			}
		}
		if errorCount > 0 {
			sb.WriteString(fmt.Sprintf("发现 %d 个错误", errorCount))
			if ec.warnings > 0 {
				sb.WriteString(fmt.Sprintf("，%d 个警告", ec.warnings))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

type Lexer struct {
	source string
	pos    int
	line   int
	col    int
	file   string
}

func NewLexer(source, filename string) *Lexer {
	return &Lexer{
		source: source,
		pos:    0,
		line:   1,
		col:    1,
		file:   filename,
	}
}

var symbolMap = map[rune]rune{
	'（': '(', '）': ')', '｛': '{', '｝': '}',
	'＂': '"', '＇': '\'', '‘': '\'', '’': '\'',
	'“': '"', '”': '"', '＝': '=', '＋': '+',
	'－': '-', '×': '*', '÷': '/', '＞': '>',
	'＜': '<', '；': ';', '，': ',', '．': '.',
	'：': ':', '＠': '@', '＃': '#',
}

func (l *Lexer) normalize(r rune) rune {
	if normalized, ok := symbolMap[r]; ok {
		return normalized
	}
	return r
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.source) {
		c := l.source[l.pos]
		if c == ' ' || c == '\t' || c == '\r' {
			l.pos++
			l.col++
		} else if c == '\n' {
			l.pos++
			l.line++
			l.col = 1
		} else {
			break
		}
	}
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()
	if l.pos >= len(l.source) {
		return Token{Type: TOKEN_EOF, Line: l.line, Column: l.col}
	}
	r, size := utf8.DecodeRuneInString(l.source[l.pos:])
	if r == utf8.RuneError {
		l.pos++
		l.col++
		return l.NextToken()
	}
	if unicode.IsDigit(r) {
		start := l.pos
		startCol := l.col
		for l.pos < len(l.source) {
			c, sz := utf8.DecodeRuneInString(l.source[l.pos:])
			if unicode.IsDigit(c) || c == '.' {
				l.pos += sz
				l.col += sz
			} else {
				break
			}
		}
		value := l.source[start:l.pos]
		return Token{Type: TOKEN_NUMBER, Value: value, Line: l.line, Column: startCol}
	}
	if unicode.IsLetter(r) || r == '_' {
		start := l.pos
		startCol := l.col
		for l.pos < len(l.source) {
			c, sz := utf8.DecodeRuneInString(l.source[l.pos:])
			if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' {
				l.pos += sz
				l.col += sz
			} else {
				break
			}
		}
		value := l.source[start:l.pos]
		return Token{Type: TOKEN_IDENTIFIER, Value: value, Line: l.line, Column: startCol}
	}
	if r == '"' || r == '\'' {
		quote := r
		l.pos += size
		l.col += size
		start := l.pos
		for l.pos < len(l.source) {
			c, sz := utf8.DecodeRuneInString(l.source[l.pos:])
			if c == quote {
				value := l.source[start:l.pos]
				l.pos += sz
				l.col += sz
				return Token{Type: TOKEN_STRING, Value: value, Line: l.line, Column: l.col - len(value) - 2}
			}
			if c == '\n' {
				l.line++
				l.col = 1
			} else {
				l.col += sz
			}
			l.pos += sz
		}
		return Token{Type: TOKEN_EOF, Line: l.line, Column: l.col}
	}
	l.pos += size
	l.col += size
	return Token{Type: TOKEN_SYMBOL, Value: string(r), Line: l.line, Column: l.col - size}
}

type Parser struct {
	lexer    *Lexer
	current  Token
	keywords map[string]string
	errors   *ErrorCollector
	filename string
	source   string
	debug    bool
}

func NewParser(source, filename string, grammar map[string][]string, debug bool) *Parser {
	keywords := make(map[string]string)
	for category, words := range grammar {
		for _, word := range words {
			keywords[word] = category
		}
	}
	return &Parser{
		lexer:    NewLexer(source, filename),
		keywords: keywords,
		errors:   NewErrorCollector(debug),
		filename: filename,
		source:   source,
		debug:    debug,
	}
}

func (p *Parser) GetErrors() *ErrorCollector {
	return p.errors
}

func (p *Parser) advance() {
	p.current = p.lexer.NextToken()
}

func (p *Parser) matchKeyword(category string) bool {
	if p.current.Type == TOKEN_IDENTIFIER {
		if cat, ok := p.keywords[p.current.Value]; ok && cat == category {
			if p.debug {
				fmt.Printf("[调试] matchKeyword: category=%s, current.Value=%s, matched=true\n", category, p.current.Value)
			}
			p.advance()
			return true
		}
		if p.debug {
			fmt.Printf("[调试] matchKeyword: category=%s, current.Value=%s, keywords[%s]=%v\n", category, p.current.Value, p.current.Value, p.keywords[p.current.Value])
		}
	}
	return false
}

func (p *Parser) matchSymbol(sym string) bool {
	if p.current.Type == TOKEN_SYMBOL && p.current.Value == sym {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expectSymbol(sym string) bool {
	if p.matchSymbol(sym) {
		return true
	}
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	p.errors.AddError(pos, "期望 '%s'，得到 '%s'", sym, p.current.Value)
	return false
}

func (p *Parser) parseNumber() (interface{}, types.Position) {
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	if p.current.Type != TOKEN_NUMBER {
		p.errors.AddError(pos, "期望数字")
		return nil, pos
	}
	val := p.current.Value
	p.advance()
	if strings.Contains(val, ".") {
		f, _ := strconv.ParseFloat(val, 64)
		return f, pos
	}
	i, _ := strconv.Atoi(val)
	return i, pos
}

func (p *Parser) parseString() (string, types.Position) {
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	if p.current.Type != TOKEN_STRING {
		p.errors.AddError(pos, "期望字符串")
		return "", pos
	}
	val := p.current.Value
	p.advance()
	return val, pos
}

func (p *Parser) parseIdentifier() (string, types.Position) {
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	if p.current.Type != TOKEN_IDENTIFIER {
		p.errors.AddError(pos, "期望标识符")
		return "", pos
	}
	val := p.current.Value
	p.advance()
	return val, pos
}

func (p *Parser) parseValue() interface{} {
	if p.current.Type == TOKEN_NUMBER {
		val, _ := p.parseNumber()
		return val
	}
	if p.current.Type == TOKEN_STRING {
		val, _ := p.parseString()
		return val
	}
	if p.current.Type == TOKEN_IDENTIFIER {
		val, _ := p.parseIdentifier()
		return map[string]interface{}{"type": "variable", "name": val}
	}
	return nil
}

func (p *Parser) Parse() *types.Program {
	p.advance()
	program := types.NewProgram()
	program.FilePath = p.filename
	program.Position = types.Position{File: p.filename, Line: 1, Column: 1}
	for p.current.Type != TOKEN_EOF {
		if p.matchKeyword("角色") {
			role := p.parseRole()
			if role != nil {
				program.AddRole(role)
			}
		} else {
			p.advance()
		}
	}
	return program
}

func (p *Parser) parseRole() *types.Role {
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	name, namePos := p.parseIdentifier()
	if name == "" {
		p.errors.AddError(namePos, "角色名不能为空")
		return nil
	}
	role := types.NewRole(name)
	role.Position = pos
	if p.debug {
		fmt.Printf("[调试] parseRole: role=%s, current=%+v\n", name, p.current)
	}
	if !p.expectSymbol("{") {
		return nil
	}
	for p.current.Type != TOKEN_EOF && !p.matchSymbol("}") {
		if p.debug {
			fmt.Printf("[调试] parseRole loop: current=%+v\n", p.current)
		}
		if p.current.Type == TOKEN_SYMBOL && p.current.Value == "@" {
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
			event := p.parseEvent()
			if event != nil {
				role.AddEvent(event)
				if p.debug {
					fmt.Printf("[调试] parseRole: 添加事件到角色, role.Events 长度=%d\n", len(role.Events))
				}
			}
			continue
		}
		if p.matchKeyword("变量") {
			varDecl := p.parseVariableDecl()
			if varDecl != nil {
				role.SetVariable(varDecl["name"].(string), varDecl["value"])
			}
			continue
		}
		p.advance()
	}
	return role
}

func (p *Parser) parseEvent() *types.Event {
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	typeToken := p.current
	if p.debug {
		fmt.Printf("[调试] parseEvent: typeToken=%+v\n", typeToken)
	}
	if typeToken.Type != TOKEN_IDENTIFIER {
		p.errors.AddError(pos, "事件需要类型")
		if p.debug {
			fmt.Printf("[调试] parseEvent: 错误 - 不是 TOKEN_IDENTIFIER\n")
		}
		return nil
	}
	p.advance()
	event := types.NewEvent(typeToken.Value)
	event.Position = pos
	if p.debug {
		fmt.Printf("[调试] parseEvent: event.Type=%s\n", event.Type)
	}
	if event.Type == "收到" || event.Type == "message" {
		if p.current.Type == TOKEN_STRING {
			msg, _ := p.parseString()
			event.Message = msg
		} else {
			p.errors.AddError(pos, "收到事件需要消息内容")
		}
	}
	if p.debug {
		fmt.Printf("[调试] parseEvent: 准备解析 { , current=%+v\n", p.current)
	}
	if !p.expectSymbol("{") {
		if p.debug {
			fmt.Printf("[调试] parseEvent: expectSymbol 返回 false，事件解析失败\n")
		}
		return nil
	}
	if p.debug {
		fmt.Printf("[调试] parseEvent: 开始解析事件内容, current=%+v\n", p.current)
	}
	for p.current.Type != TOKEN_EOF && !p.matchSymbol("}") {
		action := p.parseAction()
		if action != nil {
			event.AddAction(action)
		}
	}
	return event
}

func (p *Parser) parseAction() map[string]interface{} {
	pos := types.Position{File: p.filename, Line: p.current.Line, Column: p.current.Column}
	if p.matchKeyword("说") {
		content, contentPos := p.parseString()
		if content == "" {
			p.errors.AddError(contentPos, "说动作需要内容")
			return nil
		}
		p.matchSymbol(";")
		return map[string]interface{}{"type": "说", "content": content}
	}
	if p.matchKeyword("移动") {
		steps := p.parseValue()
		if steps == nil {
			p.errors.AddError(pos, "移动动作需要步数")
			return nil
		}
		p.matchKeyword("步")
		p.matchSymbol(";")
		return map[string]interface{}{"type": "移动", "steps": steps}
	}
	if p.matchKeyword("旋转") {
		angle := p.parseValue()
		if angle == nil {
			p.errors.AddError(pos, "旋转动作需要角度")
			return nil
		}
		p.matchKeyword("度")
		p.matchSymbol(";")
		return map[string]interface{}{"type": "旋转", "angle": angle}
	}
	if p.matchKeyword("等待") {
		seconds := p.parseValue()
		if seconds == nil {
			p.errors.AddError(pos, "等待动作需要秒数")
			return nil
		}
		p.matchKeyword("秒")
		p.matchSymbol(";")
		return map[string]interface{}{"type": "等待", "seconds": seconds}
	}
	if p.matchKeyword("广播") {
		msg, msgPos := p.parseString()
		if msg == "" {
			p.errors.AddError(msgPos, "广播动作需要消息")
			return nil
		}
		p.matchSymbol(";")
		return map[string]interface{}{"type": "广播", "message": msg}
	}
	if p.current.Type == TOKEN_IDENTIFIER {
		name, namePos := p.parseIdentifier()
		if p.matchSymbol("=") {
			value := p.parseValue()
			if value == nil {
				p.errors.AddError(namePos, "赋值语句需要值")
				return nil
			}
			p.matchSymbol(";")
			return map[string]interface{}{"type": "赋值", "variable": name, "value": value}
		}
	}
	return nil
}

func (p *Parser) parseVariableDecl() map[string]interface{} {
	name, _ := p.parseIdentifier()
	if name == "" {
		return nil
	}
	if !p.expectSymbol("=") {
		return nil
	}
	value := p.parseValue()
	if value == nil {
		return nil
	}
	p.matchSymbol(";")
	return map[string]interface{}{"name": name, "value": value}
}

type CacheEntry struct {
	Data      json.RawMessage
	RefCount  int32
	LastUsed  time.Time
	FileMod   time.Time
	FilePath  string
	SourceKey string
}

type CacheManager struct {
	entries     sync.Map
	cleanupTick *time.Ticker
	stopCleanup chan bool
	mu          sync.Mutex
	cacheDir    string
	debug       bool
}

var globalCacheManager *CacheManager
var once sync.Once

func GetCacheManager(debug bool) *CacheManager {
	once.Do(func() {
		globalCacheManager = &CacheManager{
			entries:     sync.Map{},
			cleanupTick: time.NewTicker(5 * time.Minute),
			stopCleanup: make(chan bool),
			cacheDir:    "cache",
			debug:       debug,
		}
		go globalCacheManager.startCleanup()
	})
	return globalCacheManager
}

func (cm *CacheManager) getCacheKey(filename string) string {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}
	key := strings.ReplaceAll(absPath, string(os.PathSeparator), "_")
	key = strings.ReplaceAll(key, ":", "_")
	return key + ".json"
}

func (cm *CacheManager) getCacheFilePath(key string) string {
	return filepath.Join(cm.cacheDir, key)
}

func (cm *CacheManager) AddRef(filename string) bool {
	key := cm.getCacheKey(filename)
	if entry, ok := cm.entries.Load(key); ok {
		e := entry.(*CacheEntry)
		newCount := atomic.AddInt32(&e.RefCount, 1)
		e.LastUsed = time.Now()
		info, err := os.Stat(filename)
		if err == nil && info.ModTime().After(e.FileMod) {
			if cm.debug {
				fmt.Printf("[缓存] 文件 %s 已修改，缓存失效\n", filename)
			}
			cm.entries.Delete(key)
			os.Remove(e.FilePath)
			return false
		}
		if cm.debug {
			fmt.Printf("[缓存] 增加引用 %s: 当前引用数=%d\n", filename, newCount)
		}
		return true
	}
	return false
}

func (cm *CacheManager) ReleaseRef(filename string) {
	key := cm.getCacheKey(filename)
	if entry, ok := cm.entries.Load(key); ok {
		e := entry.(*CacheEntry)
		newCount := atomic.AddInt32(&e.RefCount, -1)
		e.LastUsed = time.Now()
		if cm.debug {
			fmt.Printf("[缓存] 释放引用 %s: 剩余引用数=%d\n", filename, newCount)
		}
		if newCount < 0 && cm.debug {
			fmt.Printf("[缓存警告] 引用计数异常: %s 计数=%d\n", filename, newCount)
		}
	}
}

func (cm *CacheManager) Save(filename string, program *types.Program) error {
	key := cm.getCacheKey(filename)
	data, err := types.SaveProgramToJSON(program)
	if err != nil {
		return err
	}
	info, err := os.Stat(filename)
	if err != nil {
		info = nil
	}
	fileMod := time.Now()
	if info != nil {
		fileMod = info.ModTime()
	}
	cachePath := cm.getCacheFilePath(key)
	os.MkdirAll(cm.cacheDir, 0755)
	os.Remove(cachePath)
	err = ioutil.WriteFile(cachePath, data, 0644)
	if err != nil {
		return err
	}
	entry := &CacheEntry{
		Data:      data,
		RefCount:  0,
		LastUsed:  time.Now(),
		FileMod:   fileMod,
		FilePath:  cachePath,
		SourceKey: key,
	}
	cm.entries.Store(key, entry)
	if cm.debug {
		fmt.Printf("[缓存] 已保存: %s\n", cachePath)
	}
	return nil
}

func (cm *CacheManager) Load(filename string) (*types.Program, error) {
	key := cm.getCacheKey(filename)
	if entry, ok := cm.entries.Load(key); ok {
		e := entry.(*CacheEntry)
		info, err := os.Stat(filename)
		if err == nil && info.ModTime().After(e.FileMod) {
			if cm.debug {
				fmt.Printf("[缓存] 文件 %s 已修改，缓存失效\n", filename)
			}
			cm.entries.Delete(key)
			os.Remove(e.FilePath)
			return nil, nil
		}
		e.LastUsed = time.Now()
		var program types.Program
		err = json.Unmarshal(e.Data, &program)
		if err != nil {
			return nil, err
		}
		return &program, nil
	}
	cachePath := cm.getCacheFilePath(key)
	if _, err := os.Stat(cachePath); err != nil {
		return nil, nil
	}
	data, err := ioutil.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	srcInfo, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		return nil, err
	}
	if srcInfo.ModTime().After(cacheInfo.ModTime()) {
		os.Remove(cachePath)
		return nil, nil
	}
	entry := &CacheEntry{
		Data:      data,
		RefCount:  0,
		LastUsed:  time.Now(),
		FileMod:   srcInfo.ModTime(),
		FilePath:  cachePath,
		SourceKey: key,
	}
	cm.entries.Store(key, entry)
	var program types.Program
	err = json.Unmarshal(data, &program)
	if err != nil {
		return nil, err
	}
	return &program, nil
}

func (cm *CacheManager) startCleanup() {
	for {
		select {
		case <-cm.cleanupTick.C:
			cm.cleanup()
		case <-cm.stopCleanup:
			return
		}
	}
}

func (cm *CacheManager) cleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	now := time.Now()
	cm.entries.Range(func(key, value interface{}) bool {
		e := value.(*CacheEntry)
		count := atomic.LoadInt32(&e.RefCount)
		if count == -1 && now.Sub(e.LastUsed) > 30*time.Minute {
			if cm.debug {
				fmt.Printf("[缓存] 清理: %s (引用计数=%d, 最后使用: %v)\n",
					e.FilePath, count, e.LastUsed)
			}
			cm.entries.Delete(key)
			os.Remove(e.FilePath)
		}
		return true
	})
}

func (cm *CacheManager) StopCleanup() {
	cm.stopCleanup <- true
	cm.cleanupTick.Stop()
}

func loadGrammar() (map[string][]string, error) {
	data, err := ioutil.ReadFile("grammar.json")
	if err != nil {
		return nil, fmt.Errorf("无法读取 grammar.json: %v", err)
	}
	var config struct {
		Keywords map[string][]string `json:"keywords"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 grammar.json 失败: %v", err)
	}
	if config.Keywords == nil {
		return nil, fmt.Errorf("grammar.json 中缺少 keywords 字段")
	}
	return config.Keywords, nil
}

func parseFileInternal(filename string, grammar map[string][]string, debug bool) (*types.Program, *ErrorCollector) {
	source, err := ioutil.ReadFile(filename)
	if err != nil {
		ec := NewErrorCollector(debug)
		ec.AddError(types.Position{File: filename, Line: 1, Column: 1}, "无法读取文件: %v", err)
		return nil, ec
	}
	parser := NewParser(string(source), filename, grammar, debug)
	program := parser.Parse()
	return program, parser.errors
}

//export ParseFile
func ParseFile(filename *C.char) *C.char {
	goFilename := C.GoString(filename)

	grammar, err := loadGrammar()
	if err != nil {
		return C.CString("错误: " + err.Error())
	}
	program, errors := parseFileInternal(goFilename, grammar, false)
	if errors.HasErrors() {
		return C.CString("错误: " + errors.Report())
	}
	cacheManager := GetCacheManager(false)
	cacheManager.Save(goFilename, program)
	return C.CString("解析成功")
}

//export ParseFileDebug
func ParseFileDebug(filename *C.char) *C.char {
	goFilename := C.GoString(filename)

	grammar, err := loadGrammar()
	if err != nil {
		return C.CString("错误: " + err.Error())
	}
	program, errors := parseFileInternal(goFilename, grammar, true)
	if errors.HasErrors() {
		return C.CString("错误: " + errors.Report())
	}
	cacheManager := GetCacheManager(true)
	cacheManager.Save(goFilename, program)
	return C.CString("调试解析成功")
}

//export FreeString
func FreeString(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}

func main() {}
