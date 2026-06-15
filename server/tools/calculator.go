package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// calcArgs is the calculator's argument shape. Kept tiny on purpose.
type calcArgs struct {
	Expression string `json:"expression"`
}

const calcMaxLen = 256

// Calculator evaluates an arithmetic expression over real numbers: + - * /, unary
// minus, and parentheses, e.g. "2 + 3 * (4 - 1)" or "10 / 4" -> 2.5. It is a small
// self-contained recursive-descent evaluator — no eval of code, no I/O — so it is
// safe to feed model-produced text directly. (Real division on purpose: an LLM
// calculator returning integer-truncated division would be a footgun.)
func Calculator() Tool {
	return Tool{
		Name:        "calculator",
		Description: "Evaluate an arithmetic expression. Supports + - * / and parentheses, e.g. \"2 + 3 * (4 - 1)\".",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"expression": { "type": "string", "description": "the arithmetic expression to evaluate" }
			},
			"required": ["expression"],
			"additionalProperties": false
		}`),
		Execute: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var a calcArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			expr := strings.TrimSpace(a.Expression)
			if expr == "" {
				return "", fmt.Errorf("expression is empty")
			}
			if len(expr) > calcMaxLen {
				return "", fmt.Errorf("expression too long (max %d chars)", calcMaxLen)
			}
			v, err := evalExpr(expr)
			if err != nil {
				return "", fmt.Errorf("could not evaluate %q: %w", expr, err)
			}
			return strconv.FormatFloat(v, 'g', -1, 64), nil
		},
	}
}

// --- a tiny recursive-descent arithmetic evaluator over float64 ---

type calcParser struct {
	s   string
	pos int
}

func evalExpr(s string) (float64, error) {
	p := &calcParser{s: s}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos != len(p.s) {
		return 0, fmt.Errorf("unexpected %q at position %d", p.s[p.pos:], p.pos)
	}
	return v, nil
}

// expr := term (('+'|'-') term)*
func (p *calcParser) parseExpr() (float64, error) {
	v, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		switch p.peek() {
		case '+', '-':
			op := p.next()
			rhs, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			if op == '+' {
				v += rhs
			} else {
				v -= rhs
			}
		default:
			return v, nil
		}
	}
}

// term := factor (('*'|'/') factor)*
func (p *calcParser) parseTerm() (float64, error) {
	v, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		switch p.peek() {
		case '*', '/':
			op := p.next()
			rhs, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if op == '*' {
				v *= rhs
			} else {
				if rhs == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				v /= rhs
			}
		default:
			return v, nil
		}
	}
}

// factor := ('+'|'-') factor | '(' expr ')' | number
func (p *calcParser) parseFactor() (float64, error) {
	p.skipSpace()
	switch c := p.peek(); {
	case c == '+':
		p.next()
		return p.parseFactor()
	case c == '-':
		p.next()
		v, err := p.parseFactor()
		return -v, err
	case c == '(':
		p.next()
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if p.peek() != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.next()
		return v, nil
	default:
		return p.parseNumber()
	}
}

func (p *calcParser) parseNumber() (float64, error) {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if (c >= '0' && c <= '9') || c == '.' {
			p.pos++
		} else {
			break
		}
	}
	if p.pos == start {
		return 0, fmt.Errorf("expected a number at position %d", start)
	}
	v, err := strconv.ParseFloat(p.s[start:p.pos], 64)
	if err != nil {
		return 0, fmt.Errorf("bad number %q", p.s[start:p.pos])
	}
	return v, nil
}

func (p *calcParser) skipSpace() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

func (p *calcParser) peek() byte {
	if p.pos < len(p.s) {
		return p.s[p.pos]
	}
	return 0
}

func (p *calcParser) next() byte {
	c := p.s[p.pos]
	p.pos++
	return c
}
