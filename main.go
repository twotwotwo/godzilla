package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const (
	workdirPrefix = "gopath"
)

var sep = string(os.PathSeparator)

// sanityCheck verifies that the pkg we are trying to mutest compiles and that
// the tests pass.
func sanityCheck(gopath, pkg string) {
	{ // verify it compiles
		cmd := exec.Command("go", "build", pkg)
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		// remove any binary generated
		exec.Command("go", "clean").Run()
	}
	{ // verify tests pass
		cmd := exec.Command("go", "test", "-short", pkg)
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
	{ // verify that everything is already gofmt -s before
		dir := gopath + sep + "src" + sep + pkg
		finfos, err := ioutil.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}

		for _, finfo := range finfos {
			if !strings.HasSuffix(finfo.Name(), ".go") {
				continue
			}
			cmd := exec.Command("gofmt", "-d", dir+sep+finfo.Name())
			var b bytes.Buffer
			cmd.Stdout = &b
			if err := cmd.Run(); err != nil || b.Len() > 0 {
				fmt.Printf("gofmt your package before running godzilla\n	gofmt -s -w %s*.go\n", dir+sep)
				os.Exit(1)
			}
		}
	}
}

func main() {
	// Check that we have a GOPATH
	gopath, exists := os.LookupEnv("GOPATH")
	if !exists {
		fmt.Fprint(os.Stderr, "$GOPATH not set")
		os.Exit(1)
	}

	// find the package to mutest.
	var pkgName string
	if len(os.Args) == 2 {
		pkgName = os.Args[1]
	} else {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		// no need to use os.PathSeparator here because len(`/`) == len(`\`)
		pkgName = wd[len(gopath)+len(`/src/`):]
	}

	sanityCheck(gopath, pkgName)

	// Create a temporary location to store all the mutated code
	tmpDir, err := ioutil.TempDir("", "godzilla")
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	var workers []worker
	results := make(chan Result)
	pkgPath := gopath + sep + "src" + sep + pkgName
	// generate the mutation worker.
	for n := 0; n < runtime.NumCPU(); n++ {
		workdir := tmpDir + sep + workdirPrefix + strconv.Itoa(n)
		err := os.Mkdir(workdir, 0755)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		workers = append(workers, worker{
			mutantDir: workdir,
			execDir:   pkgPath,
			results:   results,
		})
	}

	// build all the mutators
	mutators := []Mutator{swapIfElse, ConditionalsBoundaryMutator,
		MathMutator, MathAssignMutator}
	c := make(chan Mutator, len(mutators))
	for _, mutator := range mutators {
		c <- mutator
	}
	close(c)

	// launch all mutator worker.
	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go w.Mutate(c, &wg)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var res Result
	for r := range results {
		res.alive += r.alive
		res.total += r.total
	}
	fmt.Printf("score: %d/%d = %.2f\n", res.total-res.alive, res.total, float64(res.total-res.alive)/float64(res.total))
}

type Mutator func(ast.Node, func())
type Result struct {
	alive, total int
}

type worker struct {
	// the directory of the mutated source.
	mutantDir string
	// a reference to the original source, this is so if a test reads from a
	// file in the package (like binary data) we don't break that.
	execDir string

	results chan Result
}

// Mutate starts mutating the source, it gets the mutators from the given
// channel.
func (w worker) Mutate(c chan Mutator, wg *sync.WaitGroup) {
	defer wg.Done()
	// Parse the entire package
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, w.execDir, nil, parser.AllErrors)
	if err != nil {
		// the code compiled, or one of the mutant did not invert their
		// changes correctly
		panic(err)
	}

	// find the real package we want to mutate. because both {{.}} and
	// {{.}}_test can exist in the same folder and it's a valid go package.
	// However no more than 2 package can exist in the same folder.
	spkgs := make([]*ast.Package, 0, len(pkgs))
	var pkg *ast.Package
	for _, p := range pkgs {
		if !strings.HasSuffix(p.Name, "_test") {
			pkg = p
		}
		spkgs = append(spkgs, p)
	}
	if pkg == nil {
		panic("package is nil")
	}

	var files []*ast.File
	for _, file := range pkg.Files {
		files = append(files, file)
	}

	for m := range c {
		v := &Visitor{
			mutantDir:   w.mutantDir,
			originalDir: w.execDir,
			fset:        fset,
			pkgs:        spkgs,
			mutator:     m,
		}

		ast.Walk(v, pkg)
		w.results <- Result{
			alive: v.mutantAlive,
			total: v.mutantCount,
		}
	}
}

// Visitor is a struct that runs a particular mutation case on the ast.Package.
type Visitor struct {
	// the directory that this mutant should test into.
	mutantDir string

	originalDir string
	// the Fileset, not sure what that does tbh. It's for ast.
	fset *token.FileSet

	// the packages, either len is 1 or 2, if it's 2 its because we have {{.}}
	// and {{.}}_test
	pkgs []*ast.Package

	// total number of mutant generated.
	mutantCount int

	// total number of mutant killed.
	mutantAlive int

	// this function should make a change to the ast.Node, call the 2nd argument
	// function and change it back into the original ast.Node.
	mutator func(ast.Node, func())
}

// TestMutant take the current ast.Package, writes it to a new mutant package
// and test it.
func (v *Visitor) TestMutant() {
	// write all ast file to their equivalent in the mutant dir
	for _, pkg := range v.pkgs {
		for fullFileName, astFile := range pkg.Files {
			fileName := fullFileName[strings.LastIndex(fullFileName, sep)+1:]
			file, err := os.OpenFile(v.mutantDir+sep+fileName, os.O_CREATE|os.O_RDWR, 0700)
			if err != nil {
				panic(err)
			}
			err = printer.Fprint(file, v.fset, astFile)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			// the output from ast doesn't always conform to gofmt ... so try to
			// minimize the diff a maximum by gofmting the files.
			cmd := exec.Command("gofmt", "-w", v.mutantDir+sep+fileName)
			if err := cmd.Run(); err != nil {
				fmt.Println("did not gofmted", v.mutantDir+sep+fileName, err)
				return
			}
		}
	}

	// Verify that the mutant we generated actually compiles
	cmd := exec.Command("go", "build")
	cmd.Dir = v.mutantDir
	if err := cmd.Run(); err != nil {
		fmt.Println("invalid build")
		return
	}

	// execute `go test` in that folder, the GOPATH can stay the same as the
	// callers.
	// BUG(hydroflame): when the test package is called *_test this will fail to
	// import the actual mutant, make the GOPATH var of the cmd be
	// `GOPATH=.../mutantDir:ActualGOPATH`
	cmd = exec.Command("go", "test", "-short")
	cmd.Dir = v.mutantDir
	v.mutantCount++
	if getExitCode(cmd.Run()) != 0 {
		return
	}
	v.mutantAlive++

	// make the diff
	finfos, err := ioutil.ReadDir(v.mutantDir)
	if err != nil {
		return
	}
	for _, finfo := range finfos {
		// really only check diff in non-test files
		if strings.HasSuffix(finfo.Name(), "_test.go") {
			continue
		}
		cmd := exec.Command("diff", "-u", v.originalDir+sep+finfo.Name(), v.mutantDir+sep+finfo.Name())
		//cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			fmt.Println(err)
		}
	}
}

// getExitCode returns the exit code of an error returned by os/exec.Cmd.Run()
// or zero if the error is nil.
func getExitCode(err error) int {
	if err == nil {
		return 0
	} else if e, ok := err.(*exec.ExitError); ok {
		return e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}
}

// Visit simply forwards the node to the mutator func of the visitor. This
// function makes *Visitor implement the ast.Visitor interface.
func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	v.mutator(node, v.TestMutant)
	return v
}

// swapIfElse swaps an ast node if body with the following else statement, if it
// exists, it will not swap the else if body of an if/else if node.
func swapIfElse(node ast.Node, testMutant func()) {
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
func ConditionalsBoundaryMutator(node ast.Node, testMutant func()) {
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
//	>>> to <<
func MathMutator(node ast.Node, testMutant func()) {
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
func MathAssignMutator(node ast.Node, testMutant func()) {
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
func NegateConditionalsMutator(node ast.Node, testMutant func()) {
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

// Void Method Calls Mutator
/*
The void method call mutator removes method calls to void methods.
*/

// Inline constant mutator
/*
boolean             replace the unmutated value true with false and replace the unmutated value false with true
integer byte short  replace the unmutated value 1 with 0, -1 with 1, 5 with -1 or otherwise increment the unmutated value by one. 1
long                replace the unmutated value 1 with 0, otherwise increment the unmutated value by one.
float               replace the unmutated values 1.0 and 2.0 with 0.0 and replace any other value with 1.0 2
double              replace the unmutated value 1.0 with 0.0 and replace any other value with 1.0 3
*/
