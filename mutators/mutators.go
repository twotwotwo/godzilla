package mutators

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/cover"
	"regexp"
)

// Mutators maps command line names to their mutators.
var Mutators = map[string]Desc{
	"voidrm": Desc{
		M:           VoidCallRemoverMutator,
		Name:        "voidrm",
		Description: "Removes void function call.",
	},
	"swapifelse": Desc{
		M:           SwapIfElse,
		Name:        "swapifelse",
		Description: "Swaps content of if/else statements.",
	},
	"condbound": Desc{
		M:           ConditionalsBoundaryMutator,
		Name:        "condbound",
		Description: "Adds or remove an equal sign in comparison operators.",
	},
	"math": Desc{
		M:           MathMutator,
		Name:        "math",
		Description: "Swaps various mathematical operators. (eg. + to -)",
	},
	"boolop": Desc{
		M:           BooleanOperatorsMutator,
		Name:        "boolop",
		Description: "Changes && to || and vice versa.",
	},
	"mathassign": Desc{
		M:           MathAssignMutator,
		Name:        "mathassign",
		Description: "Same as the math mutator but for assignements.",
	},
	"negcond": Desc{
		M:           NegateConditionalsMutator,
		Name:        "negcond",
		Description: "Swaps comparison operators to their inverse (eg. == to !=)",
	},
}

// Desc represents a specific description of a mutator.
type Desc struct {
	M           Mutator
	Name        string
	Description string
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

// covered returns true if the node is covered.
func covered(parseInfo ParseInfo, node ast.Node) bool {
	// only call the mutator if the code will ever be executed. Non-executed
	// code is considered alive mutants, but don't bother checking or displaying
	// the modification because code coverage shows you already what isn't
	// covered in your code.
	pos := parseInfo.FileSet.Position(node.Pos())
	for _, block := range parseInfo.CoveredBlocks {
		if block.Count > 0 &&
			(block.StartLine < pos.Line || (block.StartLine == pos.Line && pos.Column >= block.StartCol)) &&
			(block.EndLine > pos.Line || (block.EndLine == pos.Line && pos.Column <= block.EndCol)) {
			return true
		}
	}
	return false
}

// VoidCallRemoverMutator removes calls to void function/methods.
func VoidCallRemoverMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	if !covered(parseInfo, node) {
		return
	}

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
	if !covered(parseInfo, node) {
		return
	}

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

	// the condition is reached but nothing goes inside, don't mutate
	if !covered(parseInfo, ifstmt) && !covered(parseInfo, el) {
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
func ConditionalsBoundaryMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	if !covered(parseInfo, node) {
		return
	}

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
func MathMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	if !covered(parseInfo, node) {
		return
	}

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

var booleanMutatorTable = map[token.Token]token.Token{
	token.LAND: token.LOR,
	token.LOR:  token.LAND,
}

// BooleanOperatorsMutator swaps various mathematical operators.
//	&&	to	||
//	||	to	&&
func BooleanOperatorsMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	if !covered(parseInfo, node) {
		return
	}

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
func MathAssignMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	if !covered(parseInfo, node) {
		return
	}

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

var negateConditionalsMutatorTable = map[token.Token]token.Token{
	token.EQL: token.NEQ,
	token.NEQ: token.EQL,

	token.LSS: token.GEQ,
	token.GEQ: token.LSS,

	token.GTR: token.LEQ,
	token.LEQ: token.GTR,
}

// NegateConditionalsMutator negates some boolean checks
func NegateConditionalsMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	if !covered(parseInfo, node) {
		return
	}

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

func DebugInspect(parseInfo ParseInfo, node ast.Node, tester Tester) {
	assign, ok := node.(*ast.AssignStmt)
	if !ok {
		return
	}

	pos := parseInfo.FileSet.Position(assign.Pos())
	fmt.Println(pos)

	if len(assign.Lhs) == 1 {
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return
		}
		fmt.Printf("%#v\n", ident)
	}
}

// ReturnValueMutator changes various return value. (eg. numbers become zero)
func ReturnValueMutator(parseInfo ParseInfo, node ast.Node, tester Tester) {
	if !covered(parseInfo, node) {
		return
	}

	if block, ok := node.(*ast.BlockStmt); ok {
		returnValueMutator(&block.List, parseInfo, tester)
	}

	// case bodies are not considered BlockStmt.
	if casec, ok := node.(*ast.CaseClause); ok {
		returnValueMutator(&casec.Body, parseInfo, tester)
	}
}

var zeroRegexp = regexp.MustCompile(`^(0+(\.0*|))|(\.0+)$`)

func returnValueMutator(stmts *[]ast.Stmt, parseInfo ParseInfo, tester Tester) {
	for i, stmt := range *stmts {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok {
			continue
		}
		for _, expr := range ret.Results {
			switch e := expr.(type) {
			case *ast.BasicLit:
				switch e.Kind {
				case token.INT, token.FLOAT:
					repl := "0"
					if zeroRegexp.Match([]byte(e.Value)) {
						repl = "1"
					}

					old := e.Value
					e.Value = repl

					tester.Test()

					e.Value = old
				}
			case *ast.Ident:
				switch t := parseInfo.TypesInfo.Types[expr].Type.(type) {
				case *types.Basic:
					unusedAssign := &ast.AssignStmt{
						Lhs: []ast.Expr{&ast.Ident{Name: "_"}},
						Rhs: []ast.Expr{&ast.Ident{Name: e.Name}},
						Tok: token.ASSIGN, // assignment token, DEFINE
						//TokPos: token.Pos,   // position of Tok
					}
					old := *stmts
					nw := make([]ast.Stmt, len(*stmts))
					copy(nw, old)

					nw = append(nw, nil)
					copy(nw[i+1:], nw[i:])
					nw[i] = unusedAssign

					*stmts = nw

					tester.Test()

					*stmts = old
				case *types.Pointer:
					fmt.Println(t)
				case *types.Named:
					fmt.Println(t)
				default:
					fmt.Printf("unknown ident type %T\n", parseInfo.TypesInfo.Types[expr].Type)
				}
			default:
				fmt.Printf("unknown expr type %T\n", expr)
			}
		}
	}
}

var floatComparisonInverterMap = map[token.Token]token.Token{
	token.EQL: token.NEQ,
	token.NEQ: token.EQL,

	token.LSS: token.GEQ,
	token.GEQ: token.LSS,

	token.LEQ: token.GTR,
	token.GTR: token.LEQ,
}

// FloatComparisonInverter applies De Morgan's law to floating point comparison
// expressions the main job of this mutator is to uncover bad handling of NaN.
func FloatComparisonInverter(parseInfo ParseInfo, node ast.Node, tester Tester) {
	/*if expr, ok := node.(ast.Expr); ok {
		t, ok := parseInfo.TypesInfo.Types[expr]
		if !ok {
			return
		}

		b, ok := t.Type.(*types.Basic)
		if !ok {
			return
		}

		if b.Kind() != types.Bool {
			return
		}
		floatComparisonInverter(expr, parseInfo, node, tester)
	}*/
	if block, ok := node.(*ast.BlockStmt); ok {
		for i := range block.List {
			switch stmt := block.List[i].(type) {
			case *ast.AssignStmt:
				for j := range stmt.Rhs {
					t, ok := parseInfo.TypesInfo.Types[stmt.Rhs[j]]
					if !ok {
						return
					}

					basic, ok := t.Type.(*types.Basic)
					if !ok {
						return
					}

					if basic.Kind() != types.Bool {
						return
					}

					floatComparisonInverter(&stmt.Rhs[j], parseInfo, node, tester)
				}
			}
		}
	}

	if ifstmt, ok := node.(*ast.IfStmt); ok {
		floatComparisonInverter(&ifstmt.Cond, parseInfo, node, tester)
	}
}

// floatComparisonInverter takes a pointer to a expression that evaluates to a
// bool and inverts it if it's a comparison between 2 floating point (or
// something like "!(f0 > f1)")
func floatComparisonInverter(expr *ast.Expr, parseInfo ParseInfo, node ast.Node, tester Tester) {
	switch e := (*expr).(type) {
	case *ast.BinaryExpr:
		binary := e
		switch binary.Op {
		case token.LOR, token.LAND:
			// recurse
			floatComparisonInverter(&binary.X, parseInfo, node, tester)
			floatComparisonInverter(&binary.Y, parseInfo, node, tester)
		case token.EQL, token.LSS, token.GTR, token.NEQ, token.LEQ, token.GEQ:
			tx, ok := parseInfo.TypesInfo.Types[binary.X]
			if !ok {
				return
			}

			bx, ok := tx.Type.(*types.Basic)
			if !ok {
				return
			}

			if k := bx.Kind(); k != types.Float32 && k != types.Float64 {
				return
			}

			// kinda redundant but make sure we're doing something valid.
			ty, ok := parseInfo.TypesInfo.Types[binary.Y]
			if !ok {
				return
			}

			by, ok := ty.Type.(*types.Basic)
			if !ok {
				return
			}

			if by.Kind() != bx.Kind() {
				return
			}

			old := *expr

			*expr = &ast.UnaryExpr{
				Op: token.NOT,
				X: &ast.BinaryExpr{
					X:  binary.X,
					Op: floatComparisonInverterMap[binary.Op],
					Y:  binary.Y,
				},
			}

			tester.Test()

			*expr = old

			printPos(parseInfo, *expr)
		}
	case *ast.UnaryExpr:
		if e.Op != token.NOT {
			return
		}
		floatComparisonInverter(&e.X, parseInfo, node, tester)
	case *ast.ParenExpr:
		floatComparisonInverter(&e.X, parseInfo, node, tester)
	}
}

// printPos is a debug function that allows me to quickly see the position of a
// specific statement.
func printPos(parseInfo ParseInfo, n ast.Node) {
	pos := parseInfo.FileSet.Position(n.Pos())
	fmt.Println(pos.String())
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
