package scim

import (
	"fmt"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// FilterOp represents a SCIM filter operator.
type FilterOp int

const (
	FilterAnd FilterOp = iota
	FilterOr
	FilterNot
	FilterEq // eq
	FilterNe // ne
	FilterSw // sw (startsWith)
	FilterEw // ew (endsWith)
	FilterCo // co (contains)
	FilterPr // pr (present)
	FilterGt // gt
	FilterLt // lt
	FilterGe // ge
	FilterLe // le
)

// FilterNode represents a node in a SCIM filter AST.
type FilterNode struct {
	Op       FilterOp
	Left     *FilterNode // for And, Or, Not, comparison operators
	Right    *FilterNode // for And, Or
	AttrPath string      // e.g., "userName", "emails.value", "name.familyName"
	Value    string      // comparison value
}

// String returns a human-readable representation of the filter node.
func (n *FilterNode) String() string {
	if n == nil {
		return ""
	}
	switch n.Op {
	case FilterAnd:
		return fmt.Sprintf("(%s AND %s)", n.Left.String(), n.Right.String())
	case FilterOr:
		return fmt.Sprintf("(%s OR %s)", n.Left.String(), n.Right.String())
	case FilterNot:
		return fmt.Sprintf("NOT(%s)", n.Left.String())
	case FilterPr:
		return fmt.Sprintf("%s pr", n.AttrPath)
	case FilterEq, FilterNe, FilterSw, FilterEw, FilterCo, FilterGt, FilterLt, FilterGe, FilterLe:
		return fmt.Sprintf("%s %s %q", n.AttrPath, opToString(n.Op), n.Value)
	default:
		return fmt.Sprintf("%s %s %q", n.AttrPath, opToString(n.Op), n.Value)
	}
}

func opToString(op FilterOp) string {
	switch op {
	case FilterEq:
		return "eq"
	case FilterNe:
		return "ne"
	case FilterSw:
		return "sw"
	case FilterEw:
		return "ew"
	case FilterCo:
		return "co"
	case FilterGt:
		return "gt"
	case FilterLt:
		return "lt"
	case FilterGe:
		return "ge"
	case FilterLe:
		return "le"
	case FilterAnd, FilterOr, FilterNot, FilterPr:
		return "unknown"
	default:
		return "unknown"
	}
}

// ParseFilter parses a SCIM filter string into an AST.
// Supported operators: and, or, not, eq, ne, sw, ew, co, pr, gt, lt, ge, le
// Parentheses are supported for grouping.
func ParseFilter(input string) (*FilterNode, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	parser := &filterParser{
		tokens: tokenize(input),
		pos:    0,
	}
	node, err := parser.parseOr()
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ─────────────────────────────────────────────
// Tokenizer
// ─────────────────────────────────────────────

type tokenKind int

const (
	tokenLParen tokenKind = iota
	tokenRParen
	tokenString // quoted string
	tokenWord   // unquoted word (attribute name or operator)
	tokenEOF
)

type token struct {
	kind  tokenKind
	value string
}

func tokenize(input string) []token {
	var tokens []token
	i := 0
	for i < len(input) {
		ch := input[i]

		// Skip whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}

		// Parentheses
		if ch == '(' {
			tokens = append(tokens, token{kind: tokenLParen, value: "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, token{kind: tokenRParen, value: ")"})
			i++
			continue
		}

		// Quoted string
		if ch == '"' {
			j := i + 1
			for j < len(input) && input[j] != '"' {
				j++
			}
			if j >= len(input) {
				// Unterminated string - take rest
				tokens = append(tokens, token{kind: tokenString, value: input[i+1:]})
				i = len(input)
			} else {
				tokens = append(tokens, token{kind: tokenString, value: input[i+1 : j]})
				i = j + 1
			}
			continue
		}

		// Word: attribute name or operator keyword
		if isWordChar(ch) {
			j := i + 1
			for j < len(input) && isWordChar(input[j]) {
				j++
			}
			tokens = append(tokens, token{kind: tokenWord, value: input[i:j]})
			i = j
			continue
		}

		// Dot for attribute paths (e.g., emails.value)
		if ch == '.' {
			// Merge with previous word if possible
			if len(tokens) > 0 && tokens[len(tokens)-1].kind == tokenWord {
				j := i + 1
				for j < len(input) && isWordChar(input[j]) {
					j++
				}
				tokens[len(tokens)-1].value += input[i:j]
				i = j
				continue
			}
		}

		// Skip unknown characters
		i++
	}

	tokens = append(tokens, token{kind: tokenEOF})
	return tokens
}

func isWordChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '_' || ch == '-'
}

// ─────────────────────────────────────────────
// Recursive Descent Parser
// ─────────────────────────────────────────────

type filterParser struct {
	tokens []token
	pos    int
}

func (p *filterParser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *filterParser) next() token {
	t := p.peek()
	p.pos++
	return t
}

// parseOr: or_expr = and_expr ("or" and_expr)*
func (p *filterParser) parseOr() (*FilterNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()
		if t.kind == tokenWord && strings.EqualFold(t.value, "or") {
			p.next()
			right, err := p.parseAnd()
			if err != nil {
				return nil, err
			}
			left = &FilterNode{Op: FilterOr, Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

// parseAnd: and_expr = not_expr ("and" not_expr)*
func (p *filterParser) parseAnd() (*FilterNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()
		if t.kind == tokenWord && strings.EqualFold(t.value, "and") {
			p.next()
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = &FilterNode{Op: FilterAnd, Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

// parseNot: not_expr = "not" not_expr | primary
func (p *filterParser) parseNot() (*FilterNode, error) {
	t := p.peek()
	if t.kind == tokenWord && strings.EqualFold(t.value, "not") {
		p.next()
		child, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &FilterNode{Op: FilterNot, Left: child}, nil
	}
	return p.parsePrimary()
}

// parsePrimary: primary = "(" or_expr ")" | comparison
func (p *filterParser) parsePrimary() (*FilterNode, error) {
	t := p.peek()

	if t.kind == tokenLParen {
		p.next()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		// Expect closing paren
		if p.peek().kind == tokenRParen {
			p.next()
		}
		return node, nil
	}

	return p.parseComparison()
}

// parseComparison: attr_path op value | attr_path "pr"
func (p *filterParser) parseComparison() (*FilterNode, error) {
	// Read attribute path
	attrToken := p.next()
	if attrToken.kind != tokenWord {
		return nil, apperrors.NewInvalidArgumentf("expected attribute name, got %q", attrToken.value)
	}
	attrPath := attrToken.value

	// Consume dots (merge attribute paths like name.familyName)
	for p.peek().kind == tokenWord {
		// Check if next token is an operator
		nextVal := strings.ToLower(p.peek().value)
		if isOperatorKeyword(nextVal) {
			break
		}
		// It's part of the attribute path (shouldn't normally happen with proper tokenization)
		attrToken = p.next()
		attrPath += "." + attrToken.value
	}

	// Read operator
	opToken := p.next()
	if opToken.kind != tokenWord {
		return nil, apperrors.NewInvalidArgumentf("expected operator, got %q", opToken.value)
	}

	opLower := strings.ToLower(opToken.value)

	// Handle "pr" (present) - no value needed
	if opLower == "pr" {
		return &FilterNode{Op: FilterPr, AttrPath: attrPath}, nil
	}

	// Map operator string to FilterOp
	op, ok := stringToFilterOp(opLower)
	if !ok {
		return nil, apperrors.NewInvalidArgumentf("unsupported filter operator: %s", opToken.value)
	}

	// Read value
	valToken := p.next()
	if valToken.kind == tokenString {
		return &FilterNode{Op: op, AttrPath: attrPath, Value: valToken.value}, nil
	}
	if valToken.kind == tokenWord {
		return &FilterNode{Op: op, AttrPath: attrPath, Value: valToken.value}, nil
	}

	return nil, apperrors.NewInvalidArgumentf("expected value after operator %s, got %q", opToken.value, valToken.value)
}

func isOperatorKeyword(s string) bool {
	switch s {
	case "eq", "ne", "sw", "ew", "co", "pr", "gt", "lt", "ge", "le",
		"and", "or", "not":
		return true
	}
	return false
}

func stringToFilterOp(s string) (FilterOp, bool) {
	switch s {
	case "eq":
		return FilterEq, true
	case "ne":
		return FilterNe, true
	case "sw":
		return FilterSw, true
	case "ew":
		return FilterEw, true
	case "co":
		return FilterCo, true
	case "gt":
		return FilterGt, true
	case "lt":
		return FilterLt, true
	case "ge":
		return FilterGe, true
	case "le":
		return FilterLe, true
	default:
		return 0, false
	}
}
