package scim

import (
	"testing"
)

func TestParseFilter_Eq(t *testing.T) {
	node, err := ParseFilter(`userName eq "john"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterEq {
		t.Errorf("expected FilterEq, got %v", node.Op)
	}
	if node.AttrPath != "userName" {
		t.Errorf("expected attrPath=userName, got %s", node.AttrPath)
	}
	if node.Value != "john" {
		t.Errorf("expected value=john, got %s", node.Value)
	}
}

func TestParseFilter_Sw(t *testing.T) {
	node, err := ParseFilter(`displayName sw "John"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterSw {
		t.Errorf("expected FilterSw, got %v", node.Op)
	}
	if node.AttrPath != "displayName" {
		t.Errorf("expected attrPath=displayName, got %s", node.AttrPath)
	}
	if node.Value != "John" {
		t.Errorf("expected value=John, got %s", node.Value)
	}
}

func TestParseFilter_Pr(t *testing.T) {
	node, err := ParseFilter("userName pr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterPr {
		t.Errorf("expected FilterPr, got %v", node.Op)
	}
	if node.AttrPath != "userName" {
		t.Errorf("expected attrPath=userName, got %s", node.AttrPath)
	}
}

func TestParseFilter_And(t *testing.T) {
	node, err := ParseFilter(`userName eq "john" and active eq "true"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterAnd {
		t.Errorf("expected FilterAnd, got %v", node.Op)
	}
	if node.Left == nil || node.Right == nil {
		t.Fatal("expected both children to be non-nil")
	}
	if node.Left.Op != FilterEq || node.Right.Op != FilterEq {
		t.Errorf("expected both children to be FilterEq")
	}
	if node.Left.AttrPath != "userName" {
		t.Errorf("expected left attrPath=userName, got %s", node.Left.AttrPath)
	}
	if node.Right.AttrPath != "active" {
		t.Errorf("expected right attrPath=active, got %s", node.Right.AttrPath)
	}
}

func TestParseFilter_Or(t *testing.T) {
	node, err := ParseFilter(`userName eq "john" or userName eq "alice"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterOr {
		t.Errorf("expected FilterOr, got %v", node.Op)
	}
}

func TestParseFilter_Not(t *testing.T) {
	node, err := ParseFilter(`not (active eq "true")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterNot {
		t.Errorf("expected FilterNot, got %v", node.Op)
	}
	if node.Left == nil {
		t.Fatal("expected child to be non-nil")
	}
	if node.Left.Op != FilterEq {
		t.Errorf("expected child to be FilterEq, got %v", node.Left.Op)
	}
}

func TestParseFilter_ComplexAndOr(t *testing.T) {
	node, err := ParseFilter(`userName eq "john" and (active eq "true" or displayName sw "Admin")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterAnd {
		t.Errorf("expected FilterAnd, got %v", node.Op)
	}
	if node.Right.Op != FilterOr {
		t.Errorf("expected right child to be FilterOr, got %v", node.Right.Op)
	}
}

func TestParseFilter_Ne(t *testing.T) {
	node, err := ParseFilter(`active ne "true"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterNe {
		t.Errorf("expected FilterNe, got %v", node.Op)
	}
}

func TestParseFilter_Co(t *testing.T) {
	node, err := ParseFilter(`displayName co "admin"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterCo {
		t.Errorf("expected FilterCo, got %v", node.Op)
	}
}

func TestParseFilter_Ew(t *testing.T) {
	node, err := ParseFilter(`userName ew "admin"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterEw {
		t.Errorf("expected FilterEw, got %v", node.Op)
	}
}

func TestParseFilter_Gt(t *testing.T) {
	node, err := ParseFilter(`meta.lastModified gt "2024-01-01"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterGt {
		t.Errorf("expected FilterGt, got %v", node.Op)
	}
}

func TestParseFilter_AttributePath(t *testing.T) {
	node, err := ParseFilter(`emails.value eq "john@example.com"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.AttrPath != "emails.value" {
		t.Errorf("expected attrPath=emails.value, got %s", node.AttrPath)
	}
}

func TestParseFilter_Empty(t *testing.T) {
	node, err := ParseFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node != nil {
		t.Errorf("expected nil for empty filter, got %v", node)
	}
}

func TestParseFilter_UnquotedValue(t *testing.T) {
	node, err := ParseFilter(`active eq true`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Value != "true" {
		t.Errorf("expected value=true, got %s", node.Value)
	}
}

func TestParseFilter_CaseInsensitive(t *testing.T) {
	node, err := ParseFilter(`userName EQ "john"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterEq {
		t.Errorf("expected FilterEq for EQ, got %v", node.Op)
	}
}

func TestFilterNode_String(t *testing.T) {
	node := &FilterNode{
		Op:    FilterAnd,
		Left:  &FilterNode{Op: FilterEq, AttrPath: "userName", Value: "john"},
		Right: &FilterNode{Op: FilterPr, AttrPath: "active"},
	}
	s := node.String()
	if s == "" {
		t.Error("expected non-empty string representation")
	}
}

func TestFilterNode_String_Nil(t *testing.T) {
	var node *FilterNode
	if node.String() != "" {
		t.Error("expected empty string for nil node")
	}
}

func TestFilterNode_String_Or(t *testing.T) {
	node := &FilterNode{
		Op:    FilterOr,
		Left:  &FilterNode{Op: FilterEq, AttrPath: "a", Value: "1"},
		Right: &FilterNode{Op: FilterNe, AttrPath: "b", Value: "2"},
	}
	s := node.String()
	if !contains(s, "OR") {
		t.Errorf("expected OR in %q", s)
	}
}

func TestFilterNode_String_Not(t *testing.T) {
	node := &FilterNode{
		Op:   FilterNot,
		Left: &FilterNode{Op: FilterEq, AttrPath: "a", Value: "1"},
	}
	s := node.String()
	if !contains(s, "NOT") {
		t.Errorf("expected NOT in %q", s)
	}
}

func TestFilterNode_String_Pr(t *testing.T) {
	node := &FilterNode{Op: FilterPr, AttrPath: "userName"}
	s := node.String()
	if !contains(s, "pr") {
		t.Errorf("expected 'pr' in %q", s)
	}
}

func TestParseFilter_Lt(t *testing.T) {
	node, err := ParseFilter(`age lt "30"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterLt {
		t.Errorf("expected FilterLt, got %v", node.Op)
	}
}

func TestParseFilter_Ge(t *testing.T) {
	node, err := ParseFilter(`age ge "18"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterGe {
		t.Errorf("expected FilterGe, got %v", node.Op)
	}
}

func TestParseFilter_Le(t *testing.T) {
	node, err := ParseFilter(`age le "65"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterLe {
		t.Errorf("expected FilterLe, got %v", node.Op)
	}
}

func TestParseFilter_ComplexNested(t *testing.T) {
	node, err := ParseFilter(`(userName eq "john" or userName eq "alice") and active eq "true"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterAnd {
		t.Errorf("expected FilterAnd, got %v", node.Op)
	}
}

func TestParseFilter_MultipleAnd(t *testing.T) {
	node, err := ParseFilter(`a eq "1" and b eq "2" and c eq "3"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterAnd {
		t.Errorf("expected FilterAnd, got %v", node.Op)
	}
}

func TestParseFilter_CaseInsensitiveOr(t *testing.T) {
	node, err := ParseFilter(`a eq "1" OR b eq "2"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterOr {
		t.Errorf("expected FilterOr, got %v", node.Op)
	}
}

func TestParseFilter_CaseInsensitiveAnd(t *testing.T) {
	node, err := ParseFilter(`a eq "1" AND b eq "2"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterAnd {
		t.Errorf("expected FilterAnd, got %v", node.Op)
	}
}

func TestParseFilter_CaseInsensitiveNot(t *testing.T) {
	node, err := ParseFilter(`NOT (a eq "1")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterNot {
		t.Errorf("expected FilterNot, got %v", node.Op)
	}
}

func TestParseFilter_Whitespace(t *testing.T) {
	node, err := ParseFilter(`  userName   eq   "john"  `)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterEq {
		t.Errorf("expected FilterEq, got %v", node.Op)
	}
}

func TestOpToString(t *testing.T) {
	tests := []struct {
		op       FilterOp
		expected string
	}{
		{FilterEq, "eq"},
		{FilterNe, "ne"},
		{FilterSw, "sw"},
		{FilterEw, "ew"},
		{FilterCo, "co"},
		{FilterGt, "gt"},
		{FilterLt, "lt"},
		{FilterGe, "ge"},
		{FilterLe, "le"},
		{FilterAnd, "unknown"},
	}
	for _, tt := range tests {
		got := opToString(tt.op)
		if got != tt.expected {
			t.Errorf("opToString(%v) = %q, want %q", tt.op, got, tt.expected)
		}
	}
}

func TestIsOperatorKeyword(t *testing.T) {
	keywords := []string{"eq", "ne", "sw", "ew", "co", "pr", "gt", "lt", "ge", "le", "and", "or", "not"}
	for _, kw := range keywords {
		if !isOperatorKeyword(kw) {
			t.Errorf("expected %q to be an operator keyword", kw)
		}
	}
	if isOperatorKeyword("unknown") {
		t.Error("expected 'unknown' not to be an operator keyword")
	}
}

func TestStringToFilterOp(t *testing.T) {
	tests := []struct {
		s  string
		op FilterOp
		ok bool
	}{
		{"eq", FilterEq, true},
		{"ne", FilterNe, true},
		{"sw", FilterSw, true},
		{"ew", FilterEw, true},
		{"co", FilterCo, true},
		{"gt", FilterGt, true},
		{"lt", FilterLt, true},
		{"ge", FilterGe, true},
		{"le", FilterLe, true},
		{"unknown", 0, false},
	}
	for _, tt := range tests {
		op, ok := stringToFilterOp(tt.s)
		if ok != tt.ok || (ok && op != tt.op) {
			t.Errorf("stringToFilterOp(%q) = (%v, %v), want (%v, %v)", tt.s, op, ok, tt.op, tt.ok)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestFilterNode_String_ComparisonOps(t *testing.T) {
	cases := []struct {
		op   FilterOp
		want string
	}{
		{FilterEq, `userName eq "john"`},
		{FilterNe, `userName ne "john"`},
		{FilterSw, `userName sw "john"`},
		{FilterEw, `userName ew "john"`},
		{FilterCo, `userName co "john"`},
		{FilterGt, `userName gt "john"`},
		{FilterLt, `userName lt "john"`},
		{FilterGe, `userName ge "john"`},
		{FilterLe, `userName le "john"`},
	}
	for _, c := range cases {
		node := &FilterNode{Op: c.op, AttrPath: "userName", Value: "john"}
		if got := node.String(); got != c.want {
			t.Errorf("String() for %v = %q, want %q", c.op, got, c.want)
		}
	}
	if got := opToString(FilterAnd); got != "unknown" {
		t.Errorf("opToString(FilterAnd) = %q, want %q", got, "unknown")
	}
}
