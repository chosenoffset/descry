// Package parser provides lexical analysis and parsing for the Descry DSL.
// It includes a tokenizer (lexer) that breaks rule text into tokens,
// and a parser that builds an Abstract Syntax Tree (AST) for efficient evaluation.
//
// The parser supports the Descry DSL syntax:
//
//	when <condition> { <action> }
//
// Example DSL expressions:
//
//	when heap.alloc > 200MB { alert("Memory usage high") }
//	when avg(http.response_time, 5m) > 500ms { log("Slow responses") }
//	when goroutines.count > 1000 && trend(heap.alloc, 2m) > 0 { alert("Resource leak") }
//
// The lexer recognizes tokens including keywords (when, if), operators (>, <, ==, &&, ||),
// literals (strings, numbers, units like MB/GB/ms), identifiers, and delimiters.
//
// The parser builds an AST that can be evaluated efficiently during runtime monitoring.
package parser

// TokenType represents the different types of tokens in the Descry DSL
type TokenType int

const (
	// Special tokens
	ILLEGAL TokenType = iota
	EOF

	// Literals
	IDENT  // variable names, function names
	INT    // integers
	FLOAT  // floating point numbers
	STRING // string literals

	// Keywords
	WHEN
	IF

	// Operators
	ASSIGN // =
	EQ     // ==
	NOT_EQ // !=
	LT     // <
	GT     // >
	LTE    // <=
	GTE    // >=
	AND    // &&
	OR     // ||
	NOT    // !

	// Delimiters
	COMMA     // ,
	SEMICOLON // ;
	DOT       // .

	LPAREN // (
	RPAREN // )
	LBRACE // {
	RBRACE // }

	// Units
	MB // megabytes
	GB // gigabytes
	MS // milliseconds
	S  // seconds
	M  // minutes
)

// Token represents a single lexical unit in the Descry DSL with position information
type Token struct {
	// Type identifies the kind of token (keyword, operator, literal, etc.)
	Type     TokenType
	// Literal is the actual text from the source
	Literal  string
	// Position is the character offset in the input
	Position int
	// Line is the line number (1-based)
	Line     int
	// Column is the column number (1-based)
	Column   int
}

var keywords = map[string]TokenType{
	"when": WHEN,
	"if":   IF,
	"MB":   MB,
	"GB":   GB,
	"ms":   MS,
	"s":    S,
	"m":    M,
}

// Lexer performs lexical analysis on Descry DSL source text,
// converting it into a sequence of tokens for parsing.
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
	line         int
	column       int
}

// NewLexer creates a new lexer for the given Descry DSL source text
func NewLexer(input string) *Lexer {
	l := &Lexer{
		input:  input,
		line:   1,
		column: 0,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
	if l.ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	tok.Position = l.position
	tok.Line = l.line
	tok.Column = l.column

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: EQ, Literal: string(ch) + string(l.ch), Position: l.position - 1, Line: l.line, Column: l.column - 1}
		} else {
			tok = newToken(ASSIGN, l.ch, l.position, l.line, l.column)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: NOT_EQ, Literal: string(ch) + string(l.ch), Position: l.position - 1, Line: l.line, Column: l.column - 1}
		} else {
			tok = newToken(NOT, l.ch, l.position, l.line, l.column)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: LTE, Literal: string(ch) + string(l.ch), Position: l.position - 1, Line: l.line, Column: l.column - 1}
		} else {
			tok = newToken(LT, l.ch, l.position, l.line, l.column)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: GTE, Literal: string(ch) + string(l.ch), Position: l.position - 1, Line: l.line, Column: l.column - 1}
		} else {
			tok = newToken(GT, l.ch, l.position, l.line, l.column)
		}
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: AND, Literal: string(ch) + string(l.ch), Position: l.position - 1, Line: l.line, Column: l.column - 1}
		} else {
			tok = newToken(ILLEGAL, l.ch, l.position, l.line, l.column)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: OR, Literal: string(ch) + string(l.ch), Position: l.position - 1, Line: l.line, Column: l.column - 1}
		} else {
			tok = newToken(ILLEGAL, l.ch, l.position, l.line, l.column)
		}
	case ',':
		tok = newToken(COMMA, l.ch, l.position, l.line, l.column)
	case ';':
		tok = newToken(SEMICOLON, l.ch, l.position, l.line, l.column)
	case '.':
		tok = newToken(DOT, l.ch, l.position, l.line, l.column)
	case '(':
		tok = newToken(LPAREN, l.ch, l.position, l.line, l.column)
	case ')':
		tok = newToken(RPAREN, l.ch, l.position, l.line, l.column)
	case '{':
		tok = newToken(LBRACE, l.ch, l.position, l.line, l.column)
	case '}':
		tok = newToken(RBRACE, l.ch, l.position, l.line, l.column)
	case '"':
		tok.Type = STRING
		tok.Literal = l.readString()
		tok.Position = l.position
		tok.Line = l.line
		tok.Column = l.column
	case 0:
		tok.Literal = ""
		tok.Type = EOF
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = lookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			tok.Type, tok.Literal = l.readNumber()
			return tok
		} else {
			tok = newToken(ILLEGAL, l.ch, l.position, l.line, l.column)
		}
	}

	l.readChar()
	return tok
}

func newToken(tokenType TokenType, ch byte, position, line, column int) Token {
	return Token{
		Type:     tokenType,
		Literal:  string(ch),
		Position: position,
		Line:     line,
		Column:   column,
	}
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() (TokenType, string) {
	position := l.position
	tokenType := INT

	for isDigit(l.ch) {
		l.readChar()
	}

	if l.ch == '.' && isDigit(l.peekChar()) {
		tokenType = FLOAT
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	return tokenType, l.input[position:l.position]
}

func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.position]
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func lookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

func (t TokenType) String() string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case IDENT:
		return "IDENT"
	case INT:
		return "INT"
	case FLOAT:
		return "FLOAT"
	case STRING:
		return "STRING"
	case WHEN:
		return "WHEN"
	case IF:
		return "IF"
	case ASSIGN:
		return "="
	case EQ:
		return "=="
	case NOT_EQ:
		return "!="
	case LT:
		return "<"
	case GT:
		return ">"
	case LTE:
		return "<="
	case GTE:
		return ">="
	case AND:
		return "&&"
	case OR:
		return "||"
	case NOT:
		return "!"
	case COMMA:
		return ","
	case SEMICOLON:
		return ";"
	case DOT:
		return "."
	case LPAREN:
		return "("
	case RPAREN:
		return ")"
	case LBRACE:
		return "{"
	case RBRACE:
		return "}"
	case MB:
		return "MB"
	case GB:
		return "GB"
	case MS:
		return "ms"
	case S:
		return "s"
	case M:
		return "m"
	default:
		return "UNKNOWN"
	}
}
