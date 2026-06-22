use super::types::{Token, TokenType};

/// Full-width symbol normalization map
pub fn normalize_char(c: char) -> char {
    match c {
        '\u{FF08}' => '(', '\u{FF09}' => ')', '\u{FF5B}' => '{', '\u{FF5D}' => '}',
        '\u{FF02}' => '"', '\u{FF07}' => '\'', '\u{2018}' => '\'', '\u{2019}' => '\'',
        '\u{201C}' => '"', '\u{201D}' => '"', '\u{FF1D}' => '=', '\u{FF0B}' => '+',
        '\u{FF0D}' => '-', '\u{00D7}' => '*', '\u{00F7}' => '/', '\u{FF1E}' => '>',
        '\u{FF1C}' => '<', '\u{FF1B}' => ';', '\u{FF0C}' => ',', '\u{FF0E}' => '.',
        '\u{FF1A}' => ':', '\u{FF20}' => '@', '\u{FF03}' => '#',
        _ => c,
    }
}

/// Lexer - tokenizes source code (no keyword classification; that's the parser's job)
pub struct Lexer {
    source: Vec<char>,
    pos: usize,
    line: usize,
    column: usize,
}

impl Lexer {
    pub fn new(source: &str) -> Self {
        Lexer {
            source: source.chars().collect(),
            pos: 0,
            line: 1,
            column: 1,
        }
    }

    fn current(&self) -> Option<char> {
        self.source.get(self.pos).copied()
    }

    fn advance(&mut self) {
        if let Some(c) = self.current() {
            if c == '\n' {
                self.line += 1;
                self.column = 1;
            } else {
                self.column += 1;
            }
            self.pos += 1;
        }
    }

    fn skip_whitespace(&mut self) {
        while let Some(c) = self.current() {
            if c == ' ' || c == '\t' || c == '\r' {
                self.column += 1;
                self.pos += 1;
            } else if c == '\n' {
                self.line += 1;
                self.column = 1;
                self.pos += 1;
            } else {
                break;
            }
        }
    }

    fn read_string(&mut self, quote: char) -> Token {
        let start_line = self.line;
        let start_col = self.column;
        self.advance(); // consume opening quote

        let mut value = String::new();
        while let Some(c) = self.current() {
            let nc = normalize_char(c);
            if nc == quote {
                self.advance();
                break;
            }
            if c == '\\' {
                self.advance();
                if let Some(escaped) = self.current() {
                    match escaped {
                        'n' => value.push('\n'),
                        't' => value.push('\t'),
                        'r' => value.push('\r'),
                        _ => value.push(escaped),
                    }
                    self.advance();
                }
            } else {
                value.push(c);
                if c == '\n' {
                    self.line += 1;
                    self.column = 1;
                } else {
                    self.column += 1;
                }
                self.pos += 1;
            }
        }

        Token::new(TokenType::String, value, start_line, start_col)
    }

    fn read_number(&mut self) -> Token {
        let start_line = self.line;
        let start_col = self.column;
        let mut value = String::new();
        let mut has_dot = false;

        while let Some(c) = self.current() {
            if c.is_ascii_digit() {
                value.push(c);
                self.advance();
            } else if c == '.' && !has_dot {
                has_dot = true;
                value.push(c);
                self.advance();
            } else {
                break;
            }
        }

        Token::new(TokenType::Number, value, start_line, start_col)
    }

    fn read_identifier(&mut self) -> Token {
        let start_line = self.line;
        let start_col = self.column;
        let mut value = String::new();

        while let Some(c) = self.current() {
            if c.is_alphabetic() || c.is_ascii_digit() || c == '_' || c as u32 > 127 {
                value.push(c);
                self.advance();
            } else {
                break;
            }
        }

        Token::new(TokenType::Identifier, value, start_line, start_col)
    }

    pub fn next_token(&mut self) -> Token {
        self.skip_whitespace();

        if self.pos >= self.source.len() {
            return Token::new(TokenType::Eof, String::new(), self.line, self.column);
        }

        let c = self.current().unwrap();
        let nc = normalize_char(c);

        match nc {
            '"' | '\'' => self.read_string(nc),
            _ if c.is_ascii_digit() => self.read_number(),
            _ if c.is_alphabetic() || c == '_' || c as u32 > 127 => self.read_identifier(),
            _ => {
                let start_line = self.line;
                let start_col = self.column;
                self.advance();
                Token::new(TokenType::Symbol, nc.to_string(), start_line, start_col)
            }
        }
    }

    pub fn tokenize(&mut self) -> Vec<Token> {
        let mut tokens = Vec::new();
        loop {
            let token = self.next_token();
            let is_eof = token.token_type == TokenType::Eof;
            tokens.push(token);
            if is_eof { break; }
        }
        tokens
    }
}
