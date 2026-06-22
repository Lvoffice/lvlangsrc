use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// ==================== Position ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub file: String,
    pub line: usize,
    pub column: usize,
}

impl Default for Position {
    fn default() -> Self {
        Position { file: String::new(), line: 1, column: 1 }
    }
}

// ==================== Token ====================

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum TokenType {
    Eof,
    Identifier,
    Number,
    String,
    Keyword,
    Symbol,
}

#[derive(Debug, Clone)]
pub struct Token {
    pub token_type: TokenType,
    pub value: String,
    pub category: String,
    pub line: usize,
    pub column: usize,
}

impl Token {
    pub fn new(token_type: TokenType, value: String, line: usize, column: usize) -> Self {
        Token {
            token_type,
            value: value.clone(),
            category: String::new(),
            line,
            column,
        }
    }
}

// ==================== AST ====================

/// Value in AST - number, string, or variable reference
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum Value {
    Number(serde_json::Number),
    String(String),
    Variable { type_: String, name: String },
}

// Actions use HashMap<String, serde_json::Value> directly (matching Go's map[string]interface{})

/// Event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Event {
    #[serde(rename = "Type")]
    pub event_type: String,
    #[serde(rename = "Message")]
    #[serde(skip_serializing_if = "String::is_empty")]
    pub message: String,
    #[serde(rename = "Actions")]
    pub actions: Vec<HashMap<String, serde_json::Value>>,
    #[serde(rename = "Special")]
    #[serde(skip_serializing_if = "String::is_empty")]
    pub special: String,
    #[serde(rename = "Position")]
    pub position: Position,
}

/// Role
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Role {
    #[serde(rename = "Name")]
    pub name: String,
    #[serde(rename = "Events")]
    pub events: Vec<Event>,
    #[serde(rename = "Variables")]
    #[serde(skip_serializing_if = "HashMap::is_empty")]
    pub variables: HashMap<String, serde_json::Value>,
    #[serde(rename = "Position")]
    pub position: Position,
}

/// Program (AST root)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Program {
    #[serde(rename = "Roles")]
    pub roles: Vec<Role>,
    #[serde(rename = "FilePath")]
    #[serde(skip_serializing_if = "String::is_empty")]
    pub file_path: String,
    #[serde(rename = "Position")]
    pub position: Position,
}

impl Default for Program {
    fn default() -> Self {
        Program {
            roles: Vec::new(),
            file_path: String::new(),
            position: Position::default(),
        }
    }
}

// ==================== Errors ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub enum ErrorLevel {
    Info,
    Warning,
    Error,
    Fatal,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParseError {
    pub level: ErrorLevel,
    pub position: Position,
    pub message: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub context: String,
}

pub struct ErrorCollector {
    pub errors: Vec<ParseError>,
    pub debug: bool,
    pub warnings: usize,
    pub infos: usize,
}

impl ErrorCollector {
    pub fn new(debug: bool) -> Self {
        ErrorCollector {
            errors: Vec::new(),
            debug,
            warnings: 0,
            infos: 0,
        }
    }

    pub fn add_error(&mut self, pos: Position, message: String) {
        self.errors.push(ParseError {
            level: ErrorLevel::Error,
            position: pos,
            message,
            context: String::new(),
        });
    }

    pub fn add_warning(&mut self, pos: Position, message: String) {
        if self.debug {
            self.errors.push(ParseError {
                level: ErrorLevel::Warning,
                position: pos,
                message,
                context: String::new(),
            });
        } else {
            self.warnings += 1;
        }
    }

    pub fn has_errors(&self) -> bool {
        self.errors.iter().any(|e| e.level >= ErrorLevel::Error)
    }

    pub fn report(&self) -> String {
        if self.errors.is_empty() && self.warnings == 0 {
            return String::new();
        }
        let mut sb = String::new();
        if self.debug {
            for err in &self.errors {
                match err.level {
                    ErrorLevel::Error => {
                        sb.push_str(&format!("错误 {}:{}:{}: {}\n", err.position.file, err.position.line, err.position.column, err.message));
                    }
                    ErrorLevel::Warning => {
                        sb.push_str(&format!("警告 {}:{}:{}: {}\n", err.position.file, err.position.line, err.position.column, err.message));
                    }
                    ErrorLevel::Info => {
                        sb.push_str(&format!("信息 {}:{}:{}: {}\n", err.position.file, err.position.line, err.position.column, err.message));
                    }
                    ErrorLevel::Fatal => {
                        sb.push_str(&format!("致命 {}:{}:{}: {}\n", err.position.file, err.position.line, err.position.column, err.message));
                    }
                }
            }
        } else {
            let error_count = self.errors.iter().filter(|e| e.level >= ErrorLevel::Error).count();
            if error_count > 0 {
                sb.push_str(&format!("发现 {} 个错误", error_count));
                if self.warnings > 0 {
                    sb.push_str(&format!("，{} 个警告", self.warnings));
                }
                sb.push('\n');
            }
        }
        sb
    }
}
