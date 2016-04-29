package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// VerifyTestPasses verifies that the pkg we are trying to mutest passes by
// default
func VerifyTestPasses(pkg string) {
	cmd := exec.Command("go", "test", pkg)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func main() {

	// Check that we got a package to mutest
	flag.Parse()
	if args := flag.Args(); len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage of %s: %s [flags] package\n", os.Args[0], os.Args[0])
		flag.PrintDefaults()
		return
	}

	// Check that we have a GOPATH
	gopath, exists := os.LookupEnv("GOPATH")
	if !exists {
		fmt.Fprint(os.Stderr, "$GOPATH not set")
		return
	}

	// Verify that the package tests actually pass.
	pkgName := flag.Args()[0]
	VerifyTestPasses(pkgName)

	// Parse the entire package
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, gopath+"/src/"+pkgName, nil, parser.AllErrors)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	// find the real package we want to mutate. because both `x` and `x_test`
	// can exist in the same folder and it's a valid go package. However no more
	// then 2 package can exist in the same folder but the `go test` test
	// earlier will take care of this.
	spkgs := make([]*ast.Package, 0, len(pkgs))
	var pkg *ast.Package
	for _, p := range pkgs {
		if !strings.HasSuffix(p.Name, "_test") {
			pkg = p
		}
		spkgs = append(spkgs, p)
	}

	//for n := 0; n < runtime.NumCPU(); n++ {
	//go func() {

	// Get a tmp dir for that mutester
	tmpDir, err := ioutil.TempDir("", "mutester")
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	//defer os.Remove(tmpDir)

	v := &Visitor{
		tmpDir:  tmpDir,
		fset:    fset,
		pkgs:    spkgs,
		mutator: swapIfElse,
	}
	ast.Walk(v, pkg)

	fmt.Println("Mutation score: ", float64(v.mutantKill)/float64(v.mutantCount))
	//}()
	//}
}

type Visitor struct {
	tmpDir      string
	fset        *token.FileSet
	pkgs        []*ast.Package
	mutantCount int
	mutantKill  int
	mutator     func(ast.Node, func())
}

func (v *Visitor) TestMutant() {
	// create the mutant dir
	mutantDir := v.tmpDir + string(os.PathSeparator) + strconv.Itoa(v.mutantCount)
	err := os.Mkdir(mutantDir, 0700)
	if err != nil {
		panic(err)
	}
	//defer os.Remove(mutantDir)

	// write all ast file to their equivalent in the mutant dir
	for _, pkg := range v.pkgs {
		for fullFileName, astFile := range pkg.Files {
			fileName := fullFileName[strings.LastIndex(fullFileName, string(os.PathSeparator))+1:]
			file, err := os.OpenFile(mutantDir+string(os.PathSeparator)+fileName, os.O_CREATE|os.O_RDWR, 0700)
			if err != nil {
				panic(err)
			}
			err = printer.Fprint(file, v.fset, astFile)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
			}
		}
	}

	cmd := exec.Command("go", "test")
	cmd.Dir = mutantDir
	if err := cmd.Run(); err != nil {
		v.mutantCount++
		if getExitCode(err) == 0 {
			v.mutantKill++
		}
	}
}

func getExitCode(err error) int {
	if err == nil {
		return 0
	} else if e, ok := err.(*exec.ExitError); ok {
		return e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}
}

func (v *Visitor) Visit(node ast.Node) (w ast.Visitor) {
	v.mutator(node, v.TestMutant)
	return v
}

func swapIfElse(node ast.Node, testMutant func()) {
	if ifstmt, ok := node.(*ast.IfStmt); ok {
		if ifstmt.Else != nil {
			if el, ok := ifstmt.Else.(*ast.BlockStmt); ok {
				ifstmt.Else = ifstmt.Body
				ifstmt.Body = el
				testMutant()
			}
		}
	}
}
