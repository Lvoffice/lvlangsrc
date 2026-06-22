use super::lexer::Lexer;
use super::types::*;

/// Parser - parses tokens into AST
pub struct Parser {
    tokens: Vec<Token>,
    pos: usize,
    keywords: std::collections::HashMap<String, String>, // word -> category
    errors: ErrorCollector,
    filename: String,
    debug: bool,
}

impl Parser {
    pub fn new(source: &str, filename: &str, grammar_keywords: &std::collections::HashMap<String, Vec<String>>, debug: bool) -> Self {
        // Build word -> category mapping from grammar
        let mut keywords = std::collections::HashMap::new();
        for (category, words) in grammar_keywords {
            for word in words {
                keywords.insert(word.clone(), category.clone());
            }
        }

        let mut lexer = Lexer::new(source);
        let tokens = lexer.tokenize();

        if debug {
            println!("[调试] Tokens: {:?}", tokens);
        }

        Parser {
            tokens,
            pos: 0,
            keywords,
            errors: ErrorCollector::new(debug),
            filename: filename.to_string(),
            debug,
        }
    }

    fn current(&self) -> &Token {
        static FALLBACK: std::sync::LazyLock<Token> = std::sync::LazyLock::new(|| {
            Token::new(TokenType::Eof, String::new(), 0, 0)
        });
        self.tokens.get(self.pos).unwrap_or(&FALLBACK)
    }

    fn advance(&mut self) {
        self.pos += 1;
    }

    fn match_keyword(&mut self, category: &str) -> bool {
        let token = self.current();
        if token.token_type == TokenType::Identifier {
            if let Some(cat) = self.keywords.get(&token.value) {
                if cat == category {
                    if self.debug {
                        println!("[调试] matchKeyword: category={}, current.Value={}, matched=true", category, token.value);
                    }
                    self.advance();
                    return true;
                }
            }
            if self.debug {
                println!("[调试] matchKeyword: category={}, current.Value={}, not matched", category, token.value);
            }
        }
        false
    }

    fn match_symbol(&mut self, sym: &str) -> bool {
        if self.current().token_type == TokenType::Symbol && self.current().value == sym {
            self.advance();
            return true;
        }
        false
    }

    fn expect_symbol(&mut self, sym: &str) -> bool {
        if self.match_symbol(sym) {
            return true;
        }
        let pos = Position {
            file: self.filename.clone(),
            line: self.current().line,
            column: self.current().column,
        };
        self.errors.add_error(pos, format!("期望 '{}'，得到 '{}'", sym, self.current().value));
        false
    }

    fn parse_number(&mut self) -> Option<serde_json::Value> {
        let pos = Position {
            file: self.filename.clone(),
            line: self.current().line,
            column: self.current().column,
        };
        if self.current().token_type != TokenType::Number {
            self.errors.add_error(pos, "期望数字".to_string());
            return None;
        }
        let val = self.current().value.clone();
        self.advance();
        if val.contains('.') {
            val.parse::<f64>().ok().map(|f| serde_json::json!(f))
        } else {
            val.parse::<i64>().ok().map(|i| serde_json::json!(i))
        }
    }

    fn parse_string(&mut self) -> Option<String> {
        let pos = Position {
            file: self.filename.clone(),
            line: self.current().line,
            column: self.current().column,
        };
        if self.current().token_type != TokenType::String {
            self.errors.add_error(pos, "期望字符串".to_string());
            return None;
        }
        let val = self.current().value.clone();
        self.advance();
        Some(val)
    }

    fn parse_identifier(&mut self) -> Option<String> {
        let pos = Position {
            file: self.filename.clone(),
            line: self.current().line,
            column: self.current().column,
        };
        if self.current().token_type != TokenType::Identifier {
            self.errors.add_error(pos, "期望标识符".to_string());
            return None;
        }
        let val = self.current().value.clone();
        self.advance();
        Some(val)
    }

    /// parseValue: number | string | variable reference
    fn parse_value(&mut self) -> Option<serde_json::Value> {
        match self.current().token_type {
            TokenType::Number => self.parse_number(),
            TokenType::String => self.parse_string().map(|s| serde_json::json!(s)),
            TokenType::Identifier => {
                let name = self.parse_identifier()?;
                Some(serde_json::json!({"type": "variable", "name": name}))
            }
            _ => None,
        }
    }

    /// Parse entire program
    pub fn parse(&mut self) -> Result<Program, ErrorCollector> {
        let mut program = Program::default();
        program.file_path = self.filename.clone();
        program.position = Position {
            file: self.filename.clone(),
            line: 1,
            column: 1,
        };

        while self.current().token_type != TokenType::Eof {
            if self.match_keyword("角色") {
                if let Some(role) = self.parse_role() {
                    program.roles.push(role);
                }
            } else {
                self.advance();
            }
        }

        if self.errors.has_errors() {
            return Err(std::mem::replace(&mut self.errors, ErrorCollector::new(false)));
        }
        Ok(program)
    }

    fn parse_role(&mut self) -> Option<Role> {
        let pos = Position {
            file: self.filename.clone(),
            line: self.current().line,
            column: self.current().column,
        };
        let name = self.parse_identifier().unwrap_or_default();
        if name.is_empty() {
            self.errors.add_error(pos, "角色名不能为空".to_string());
            return None;
        }

        let mut role = Role {
            name,
            events: Vec::new(),
            variables: std::collections::HashMap::new(),
            position: pos,
        };

        if self.debug {
            println!("[调试] parseRole: role={}", role.name);
        }

        if !self.expect_symbol("{") {
            return None;
        }

        while self.current().token_type != TokenType::Eof && !self.match_symbol("}") {
            if self.debug {
                println!("[调试] parseRole loop: current={:?}", self.current());
            }

            // @ special event
            if self.current().token_type == TokenType::Symbol && self.current().value == "@" {
                self.advance();
                if self.match_keyword("事件") {
                    if let Some(mut event) = self.parse_event() {
                        event.special = "@".to_string();
                        role.events.push(event);
                    }
                }
                continue;
            }

            if self.match_keyword("事件") {
                if let Some(event) = self.parse_event() {
                    role.events.push(event);
                    if self.debug {
                        println!("[调试] parseRole: 添加事件, events 长度={}", role.events.len());
                    }
                }
                continue;
            }

            if self.match_keyword("变量") {
                if let Some((var_name, var_value)) = self.parse_variable_decl() {
                    role.variables.insert(var_name, var_value);
                }
                continue;
            }

            self.advance();
        }

        Some(role)
    }

    fn parse_event(&mut self) -> Option<Event> {
        let pos = Position {
            file: self.filename.clone(),
            line: self.current().line,
            column: self.current().column,
        };

        let type_token = self.current().clone();
        if self.debug {
            println!("[调试] parseEvent: typeToken={:?}", type_token);
        }

        if type_token.token_type != TokenType::Identifier {
            self.errors.add_error(pos.clone(), "事件需要类型".to_string());
            return None;
        }

        let event_type = type_token.value.clone();
        self.advance();

        let mut event = Event {
            event_type,
            message: String::new(),
            actions: Vec::new(),
            special: String::new(),
            position: pos.clone(),
        };

        if self.debug {
            println!("[调试] parseEvent: event.Type={}", event.event_type);
        }

        // "收到"/"message" events have a message parameter
        if event.event_type == "收到" || event.event_type == "message" {
            if self.current().token_type == TokenType::String {
                if let Some(msg) = self.parse_string() {
                    event.message = msg;
                }
            } else {
                self.errors.add_error(pos, "收到事件需要消息内容".to_string());
            }
        }

        if self.debug {
            println!("[调试] parseEvent: 准备解析 {{, current={:?}", self.current());
        }

        if !self.expect_symbol("{") {
            return None;
        }

        if self.debug {
            println!("[调试] parseEvent: 开始解析事件内容, current={:?}", self.current());
        }

        while self.current().token_type != TokenType::Eof && !self.match_symbol("}") {
            if let Some(action) = self.parse_action() {
                event.actions.push(action);
            }
        }

        Some(event)
    }

    fn parse_action(&mut self) -> Option<std::collections::HashMap<String, serde_json::Value>> {
        let pos = Position {
            file: self.filename.clone(),
            line: self.current().line,
            column: self.current().column,
        };

        if self.match_keyword("说") {
            if let Some(content) = self.parse_string() {
                if content.is_empty() {
                    self.errors.add_error(pos, "说动作需要内容".to_string());
                    return None;
                }
                self.match_symbol(";");
                let mut m = std::collections::HashMap::new();
                m.insert("type".to_string(), serde_json::json!("说"));
                m.insert("content".to_string(), serde_json::json!(content));
                return Some(m);
            }
            return None;
        }

        if self.match_keyword("移动") {
            let steps = self.parse_value();
            if steps.is_none() {
                self.errors.add_error(pos, "移动动作需要步数".to_string());
                return None;
            }
            self.match_keyword("步");
            self.match_symbol(";");
            let mut m = std::collections::HashMap::new();
            m.insert("type".to_string(), serde_json::json!("移动"));
            m.insert("steps".to_string(), steps.unwrap());
            return Some(m);
        }

        if self.match_keyword("旋转") {
            let angle = self.parse_value();
            if angle.is_none() {
                self.errors.add_error(pos, "旋转动作需要角度".to_string());
                return None;
            }
            self.match_keyword("度");
            self.match_symbol(";");
            let mut m = std::collections::HashMap::new();
            m.insert("type".to_string(), serde_json::json!("旋转"));
            m.insert("angle".to_string(), angle.unwrap());
            return Some(m);
        }

        if self.match_keyword("等待") {
            let seconds = self.parse_value();
            if seconds.is_none() {
                self.errors.add_error(pos, "等待动作需要秒数".to_string());
                return None;
            }
            self.match_keyword("秒");
            self.match_symbol(";");
            let mut m = std::collections::HashMap::new();
            m.insert("type".to_string(), serde_json::json!("等待"));
            m.insert("seconds".to_string(), seconds.unwrap());
            return Some(m);
        }

        if self.match_keyword("广播") {
            if let Some(msg) = self.parse_string() {
                if msg.is_empty() {
                    self.errors.add_error(pos, "广播动作需要消息".to_string());
                    return None;
                }
                self.match_symbol(";");
                let mut m = std::collections::HashMap::new();
                m.insert("type".to_string(), serde_json::json!("广播"));
                m.insert("message".to_string(), serde_json::json!(msg));
                return Some(m);
            }
            return None;
        }

        // Assignment: identifier = value ;
        if self.current().token_type == TokenType::Identifier {
            let name = self.current().value.clone();
            let name_pos = Position {
                file: self.filename.clone(),
                line: self.current().line,
                column: self.current().column,
            };
            self.advance();
            if self.match_symbol("=") {
                let value = self.parse_value();
                if value.is_none() {
                    self.errors.add_error(name_pos, "赋值语句需要值".to_string());
                    return None;
                }
                self.match_symbol(";");
                let mut m = std::collections::HashMap::new();
                m.insert("type".to_string(), serde_json::json!("赋值"));
                m.insert("variable".to_string(), serde_json::json!(name));
                m.insert("value".to_string(), value.unwrap());
                return Some(m);
            }
        }

        // Skip semicolons
        if self.match_symbol(";") {
            return None;
        }

        self.advance();
        None
    }

    fn parse_variable_decl(&mut self) -> Option<(String, serde_json::Value)> {
        let name = self.parse_identifier()?;
        if name.is_empty() {
            return None;
        }
        if !self.expect_symbol("=") {
            return None;
        }
        let value = self.parse_value()?;
        self.match_symbol(";");
        Some((name, value))
    }
}
