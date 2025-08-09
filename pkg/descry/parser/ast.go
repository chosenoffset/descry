package parser

import (
	"bytes"
	"strings"
)

type Node interface {
	TokenLiteral() string
	String() string
}

// NodeCounter interface for efficient AST node counting
type NodeCounter interface {
	CountNodes() int
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

func (p *Program) CountNodes() int {
	count := 1 // Count the program node itself
	for _, stmt := range p.Statements {
		if counter, ok := stmt.(NodeCounter); ok {
			count += counter.CountNodes()
		} else {
			count += 1 // Fallback for nodes that don't implement NodeCounter
		}
	}
	return count
}

type WhenStatement struct {
	Token     Token // the 'when' token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhenStatement) statementNode()       {}
func (ws *WhenStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhenStatement) String() string {
	var out bytes.Buffer
	out.WriteString(ws.TokenLiteral())
	out.WriteString(" ")
	if ws.Condition != nil {
		out.WriteString(ws.Condition.String())
	}
	out.WriteString(" ")
	if ws.Body != nil {
		out.WriteString(ws.Body.String())
	}
	return out.String()
}

func (ws *WhenStatement) CountNodes() int {
	count := 1 // Count the when statement itself
	if ws.Condition != nil {
		if counter, ok := ws.Condition.(NodeCounter); ok {
			count += counter.CountNodes()
		} else {
			count += 1
		}
	}
	if ws.Body != nil {
		// BlockStatement implements NodeCounter, so we can call it directly
		count += ws.Body.CountNodes()
	}
	return count
}

type BlockStatement struct {
	Token      Token // the '{' token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	out.WriteString("{")
	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}
	out.WriteString("}")
	return out.String()
}

func (bs *BlockStatement) CountNodes() int {
	count := 1 // Count the block statement itself
	for _, stmt := range bs.Statements {
		if counter, ok := stmt.(NodeCounter); ok {
			count += counter.CountNodes()
		} else {
			count += 1
		}
	}
	return count
}

type ExpressionStatement struct {
	Token      Token // the first token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

type Identifier struct {
	Token Token // the token.IDENT token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }
func (i *Identifier) CountNodes() int { return 1 }

type IntegerLiteral struct {
	Token Token // the token.INT token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }
func (il *IntegerLiteral) CountNodes() int { return 1 }

type FloatLiteral struct {
	Token Token // the token.FLOAT token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }
func (fl *FloatLiteral) CountNodes() int { return 1 }

type StringLiteral struct {
	Token Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return sl.Token.Literal }
func (sl *StringLiteral) CountNodes() int { return 1 }

type UnitExpression struct {
	Token Token // the unit token (MB, GB, ms, etc.)
	Value Expression
	Unit  string
}

func (ue *UnitExpression) expressionNode()      {}
func (ue *UnitExpression) TokenLiteral() string { return ue.Token.Literal }
func (ue *UnitExpression) String() string {
	var out bytes.Buffer
	if ue.Value != nil {
		out.WriteString(ue.Value.String())
	}
	out.WriteString(ue.Unit)
	return out.String()
}

func (ue *UnitExpression) CountNodes() int {
	count := 1 // Count the unit expression itself
	if ue.Value != nil {
		if counter, ok := ue.Value.(NodeCounter); ok {
			count += counter.CountNodes()
		} else {
			count += 1
		}
	}
	return count
}

type InfixExpression struct {
	Token    Token // the operator token, e.g. +, -, *, /, ==, !=, <, >, <=, >=
	Left     Expression
	Operator string
	Right    Expression
}

func (oe *InfixExpression) expressionNode()      {}
func (oe *InfixExpression) TokenLiteral() string { return oe.Token.Literal }
func (oe *InfixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	if oe.Left != nil {
		out.WriteString(oe.Left.String())
	}
	out.WriteString(" " + oe.Operator + " ")
	if oe.Right != nil {
		out.WriteString(oe.Right.String())
	}
	out.WriteString(")")
	return out.String()
}

func (oe *InfixExpression) CountNodes() int {
	count := 1 // Count the infix expression itself
	if oe.Left != nil {
		if counter, ok := oe.Left.(NodeCounter); ok {
			count += counter.CountNodes()
		} else {
			count += 1
		}
	}
	if oe.Right != nil {
		if counter, ok := oe.Right.(NodeCounter); ok {
			count += counter.CountNodes()
		} else {
			count += 1
		}
	}
	return count
}

type PrefixExpression struct {
	Token    Token // the prefix token, e.g. !, -
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(pe.Operator)
	if pe.Right != nil {
		out.WriteString(pe.Right.String())
	}
	out.WriteString(")")
	return out.String()
}

type CallExpression struct {
	Token     Token // the '(' token
	Function  Expression // Identifier or FunctionLiteral
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer
	var args []string
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}
	if ce.Function != nil {
		out.WriteString(ce.Function.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

func (ce *CallExpression) CountNodes() int {
	count := 1 // Count the call expression itself
	if ce.Function != nil {
		if counter, ok := ce.Function.(NodeCounter); ok {
			count += counter.CountNodes()
		} else {
			count += 1
		}
	}
	for _, arg := range ce.Arguments {
		if counter, ok := arg.(NodeCounter); ok {
			count += counter.CountNodes()
		} else {
			count += 1
		}
	}
	return count
}

type DotExpression struct {
	Token Token // the '.' token
	Left  Expression
	Right Expression
}

func (de *DotExpression) expressionNode()      {}
func (de *DotExpression) TokenLiteral() string { return de.Token.Literal }
func (de *DotExpression) String() string {
	var out bytes.Buffer
	if de.Left != nil {
		out.WriteString(de.Left.String())
	}
	out.WriteString(".")
	if de.Right != nil {
		out.WriteString(de.Right.String())
	}
	return out.String()
}