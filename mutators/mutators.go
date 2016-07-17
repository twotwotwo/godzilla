package mutators

import (
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/cover"
)

// Mutators maps command line names to their mutators.
var Mutators = map[string]Mutator{
	"voidrm":     VoidCallRemoverMutator,
	"swapifelse": SwapIfElse,
	"condbound":  ConditionalsBoundaryMutator,
	"math":       MathMutator,
	"boolop":     BooleanOperatorsMutator,
	"mathassign": MathAssignMutator,
	"negcond":    NegateConditionalsMutator,
}

// Tester represents an interface that allows mutators to test their mutation.
// The passed Tester needs to keep track of wether the mutant passed the tests
// or not
type Tester interface {
	Test()
}

// FuncTester implements Tester, just a shortcut for functions that don't need a
// receiver.
type FuncTester func()

// Test tests the mutant.
func (f FuncTester) Test() {
	f()
}

// Mutator is an operation that can be applied to go source to mutate it.
type Mutator func(ParseInfo, ast.Node, Tester)

// ParseInfo is the information about the parsed package we are trying to
// mutate.
type ParseInfo struct {
	FileSet       *token.FileSet
	CoveredBlocks []cover.ProfileBlock
	TypesInfo     *types.Info
}

func coverageFilter(parseInfo ParseInfo, node ast.Node, tester Tester, mutator Mutator) {
	// only call the mutator if the code will ever be executed. Non-executed
	// code is considered alive mutants, but don't bother checking or displaying
	// the modification because code coverage shows you already what isn't
	// covered in your code.
	pos := parseInfo.FileSet.Position(node.Pos())
	for _, block := range parseInfo.CoveredBlocks {
		if block.Count > 0 &&
			(block.StartLine < pos.Line || (block.StartLine == pos.Line && pos.Column >= block.StartCol)) &&
			(block.EndLine > pos.Line || (block.EndLine == pos.Line && pos.Column <= block.EndCol)) {
			mutator(parseInfo, node, tester)
		}
	}
}

// VoidCallRemoverMutator removes calls to void function/methods.
func VoidCallRemoverMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	coverageFilter(parseInfo, node, tester, voidCallRemoverMutator)
}

func voidCallRemoverMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	block, ok := node.(*ast.BlockStmt)
	if !ok {
		return
	}
	for i, stmt := range block.List {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}

		v, ok := parseInfo.TypesInfo.Types[expr.X]
		if !ok {

		}

		if !v.IsVoid() {
			continue
		}

		mutation := make([]ast.Stmt, len(block.List))
		copy(mutation, block.List)
		mutation = mutation[:i+copy(mutation[i:], mutation[i+1:])]
		old := block.List
		block.List = mutation

		tester.Test()

		block.List = old
	}
}

// SwapIfElse swaps an ast node if body with the following else statement, if it
// exists, it will not swap the else if body of an if/else if node.
func SwapIfElse(parseInfo ParseInfo, node ast.Node, tester Tester) {
	coverageFilter(parseInfo, node, tester, swapIfElse)
}

// swapIfElse swaps an ast node if body with the following else statement, if it
// exists, it will not swap the else if body of an if/else if node.
func swapIfElse(_ ParseInfo, node ast.Node, tester Tester) {
	// if its an if statement node
	ifstmt, ok := node.(*ast.IfStmt)
	if !ok {
		return
	}
	// if theres an else
	if ifstmt.Else == nil {
		return
	}
	// if the else is not part of a elseif
	el, ok := ifstmt.Else.(*ast.BlockStmt)
	if !ok {
		return
	}
	// swap their body
	ifstmt.Else = ifstmt.Body
	ifstmt.Body = el
	// test that mutant
	tester.Test()
	// swap back
	ifstmt.Body = ifstmt.Else.(*ast.BlockStmt)
	ifstmt.Else = el
}

// ConditionalsBoundaryMutator performs
//	<  to <=
//	<= to <
//	>  to >=
//	>= to >
func ConditionalsBoundaryMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	coverageFilter(parseInfo, node, tester, conditionalsBoundaryMutator)
}

var conditionalsBoundaryMutatorTable = map[token.Token]token.Token{
	token.LSS: token.LEQ,
	token.LEQ: token.LSS,
	token.GTR: token.GEQ,
	token.GEQ: token.GTR,
}

func conditionalsBoundaryMutator(_ ParseInfo, node ast.Node, tester Tester) {
	expr, ok := node.(*ast.BinaryExpr)
	if !ok {
		return
	}

	old := expr.Op
	op, ok := conditionalsBoundaryMutatorTable[expr.Op]
	if !ok {
		return
	}
	expr.Op = op

	tester.Test()

	expr.Op = old
}

// MathMutator swaps various mathematical operators
//	+   to -
//	-   to +
//	*   to /
//	/   to *
//	%   to *
//	&   to |
//	|   to &
//	^   to &
//	<<  to >>
//	>>  to <<
func MathMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	coverageFilter(parseInfo, node, tester, mathMutator)
}

var mathMutatorTable = map[token.Token]token.Token{
	token.ADD: token.SUB,
	token.SUB: token.ADD,

	token.MUL: token.QUO,
	token.QUO: token.MUL,

	token.REM: token.MUL,

	token.AND: token.OR,
	token.OR:  token.AND,

	token.XOR: token.AND,

	token.SHL: token.SHR,
	token.SHR: token.SHL,
}

func mathMutator(_ ParseInfo, node ast.Node, tester Tester) {
	expr, ok := node.(*ast.BinaryExpr)
	if !ok {
		return
	}

	old := expr.Op
	op, ok := mathMutatorTable[expr.Op]
	if !ok {
		return
	}
	expr.Op = op

	tester.Test()

	expr.Op = old
}

// BooleanOperatorsMutator swaps various mathematical operators.
//	&&	to	||
//	||	to	&&
func BooleanOperatorsMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	coverageFilter(parseInfo, node, tester, booleanOperatorsMutator)
}

var booleanMutatorTable = map[token.Token]token.Token{
	token.LAND: token.LOR,
	token.LOR:  token.LAND,
}

func booleanOperatorsMutator(_ ParseInfo, node ast.Node, tester Tester) {
	expr, ok := node.(*ast.BinaryExpr)
	if !ok {
		return
	}

	old := expr.Op
	op, ok := booleanMutatorTable[expr.Op]
	if !ok {
		return
	}
	expr.Op = op

	tester.Test()

	expr.Op = old
}

// MathAssignMutator acts like MathMutator but on assignements.
func MathAssignMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	coverageFilter(parseInfo, node, tester, mathAssignMutator)
}

var mathAssignementMutatorTable = map[token.Token]token.Token{
	token.ADD_ASSIGN: token.SUB_ASSIGN,
	token.SUB_ASSIGN: token.ADD_ASSIGN,

	token.MUL_ASSIGN: token.QUO_ASSIGN,
	token.QUO_ASSIGN: token.MUL_ASSIGN,

	token.REM_ASSIGN: token.MUL_ASSIGN,

	token.AND_ASSIGN: token.OR_ASSIGN,
	token.OR_ASSIGN:  token.AND_ASSIGN,

	token.XOR_ASSIGN: token.AND_ASSIGN,

	token.SHL_ASSIGN: token.SHR_ASSIGN,
	token.SHR_ASSIGN: token.SHL_ASSIGN,
}

func mathAssignMutator(_ ParseInfo, node ast.Node, tester Tester) {
	assign, ok := node.(*ast.AssignStmt)
	if !ok {
		return
	}

	old := assign.Tok
	op, ok := mathAssignementMutatorTable[assign.Tok]
	if !ok {
		return
	}
	assign.Tok = op

	tester.Test()

	assign.Tok = old
}

// NegateConditionalsMutator negates some boolean checks
func NegateConditionalsMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	coverageFilter(parseInfo, node, tester, negateConditionalsMutator)
}

var negateConditionalsMutatorTable = map[token.Token]token.Token{
	token.EQL: token.NEQ,
	token.NEQ: token.EQL,

	token.LSS: token.GEQ,
	token.GEQ: token.LSS,

	token.GTR: token.LEQ,
	token.LEQ: token.GTR,
}

func negateConditionalsMutator(_ ParseInfo, node ast.Node, tester Tester) {
	expr, ok := node.(*ast.BinaryExpr)
	if !ok {
		return
	}

	old := expr.Op
	op, ok := negateConditionalsMutatorTable[expr.Op]
	if !ok {
		return
	}
	expr.Op = op

	tester.Test()

	expr.Op = old
}

// Increments Mutator
/*
++
--
*/

// Invert Negatives Mutator
/*
i => -i
*/

// Return Values Mutator
/*
boolean         replace the unmutated return value true with false and replace the unmutated return value false with true
int byte short  if the unmutated return value is 0 return 1, otherwise mutate to return value 0
long            replace the unmutated return value x with the result of x+1
float double    replace the unmutated return value x with the result of -(x+1.0) if x is not NAN and replace NAN with 0
Object          replace non-null return values with null and throw a java.lang.RuntimeException if the unmutated method would return null
*/

// Inline constant mutator
/*
boolean             replace the unmutated value true with false and replace the unmutated value false with true
integer byte short  replace the unmutated value 1 with 0, -1 with 1, 5 with -1 or otherwise increment the unmutated value by one. 1
long                replace the unmutated value 1 with 0, otherwise increment the unmutated value by one.
float               replace the unmutated values 1.0 and 2.0 with 0.0 and replace any other value with 1.0 2
double              replace the unmutated value 1.0 with 0.0 and replace any other value with 1.0 3
*/
