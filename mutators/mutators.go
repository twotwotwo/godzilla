package mutators

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Mutator is an operation that can be applied to go source to mutate it.
type Mutator func(*types.Info, ast.Node, func())

// VoidCallRemoverMutator removes calls to void function/methods
func VoidCallRemoverMutator(v *types.Info, node ast.Node, testMutant func()) {
	if block, ok := node.(*ast.BlockStmt); ok {
		for i, stmt := range block.List {
			if expr, ok := stmt.(*ast.ExprStmt); ok {
				if v, ok := v.Types[expr.X]; ok {
					if v.IsVoid() {
						mutation := make([]ast.Stmt, len(block.List))
						copy(mutation, block.List)
						mutation = mutation[:i+copy(mutation[i:], mutation[i+1:])]
						old := block.List
						block.List = mutation
						testMutant()
						block.List = old
					}
				}
			}
		}
	}
}

// SwapIfElse swaps an ast node if body with the following else statement, if it
// exists, it will not swap the else if body of an if/else if node.
func SwapIfElse(_ *types.Info, node ast.Node, testMutant func()) {
	// if its an if statement node
	if ifstmt, ok := node.(*ast.IfStmt); ok {
		// if theres an else
		if ifstmt.Else != nil {
			// if the else is not part of a elseif
			if el, ok := ifstmt.Else.(*ast.BlockStmt); ok {
				// swap their body
				ifstmt.Else = ifstmt.Body
				ifstmt.Body = el
				// test that mutant
				testMutant()
				// swap back
				ifstmt.Body = ifstmt.Else.(*ast.BlockStmt)
				ifstmt.Else = el
			}
		}
	}
}

var conditionalsBoundaryMutatorTable = map[token.Token]token.Token{
	token.LSS: token.LEQ,
	token.LEQ: token.LSS,
	token.GTR: token.GEQ,
	token.GEQ: token.GTR,
}

// ConditionalsBoundaryMutator performs
//	<  to <=
//	<= to <
//	>  to >=
//	>= to >
func ConditionalsBoundaryMutator(_ *types.Info, node ast.Node, testMutant func()) {
	if expr, ok := node.(*ast.BinaryExpr); ok {
		old := expr.Op
		op, ok := conditionalsBoundaryMutatorTable[expr.Op]
		if !ok {
			return
		}
		expr.Op = op
		testMutant()
		expr.Op = old
	}
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
func MathMutator(_ *types.Info, node ast.Node, testMutant func()) {
	if expr, ok := node.(*ast.BinaryExpr); ok {
		old := expr.Op
		op, ok := mathMutatorTable[expr.Op]
		if !ok {
			return
		}
		expr.Op = op
		testMutant()
		expr.Op = old
	}
}

var booleanMutatorTable = map[token.Token]token.Token{
	token.LAND: token.LOR,
	token.LOR:  token.LAND,
}

// BooleanOperatorsMutator swaps various mathematical operators.
//	&&	to	||
//	||	to	&&
func BooleanOperatorsMutator(_ *types.Info, node ast.Node, testMutant func()) {
	if expr, ok := node.(*ast.BinaryExpr); ok {
		old := expr.Op
		op, ok := booleanMutatorTable[expr.Op]
		if !ok {
			return
		}
		expr.Op = op
		testMutant()
		expr.Op = old
	}
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

// MathAssignMutator acts like MathMutator but on assignements.
func MathAssignMutator(_ *types.Info, node ast.Node, testMutant func()) {
	if assign, ok := node.(*ast.AssignStmt); ok {
		old := assign.Tok
		op, ok := mathAssignementMutatorTable[assign.Tok]
		if !ok {
			return
		}
		assign.Tok = op
		testMutant()
		assign.Tok = old
	}
}

var negateConditionalsMutatorTable = map[token.Token]token.Token{
	token.EQL: token.NEQ,
	token.NEQ: token.EQL,

	token.LSS: token.GEQ,
	token.GEQ: token.LSS,

	token.GTR: token.LEQ,
	token.LEQ: token.GTR,
}

// NegateConditionalsMutator negates some boolean checks
func NegateConditionalsMutator(_ *types.Info, node ast.Node, testMutant func()) {
	if expr, ok := node.(*ast.BinaryExpr); ok {
		old := expr.Op
		op, ok := negateConditionalsMutatorTable[expr.Op]
		if !ok {
			return
		}
		expr.Op = op
		testMutant()
		expr.Op = old
	}
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

// Math Mutator
/*
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
