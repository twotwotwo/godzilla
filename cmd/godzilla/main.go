package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/hydroflame/godzilla/mutators"
	"golang.org/x/tools/cover"
)

type config struct {
	// The importable name of the package to irradiate.
	pkg string

	// The full system path to the target package
	pkgFull string

	// A reference to the user gopath
	gopath string
}

func getRunConfig() config {
	// Check that we have a GOPATH
	gopath, exists := os.LookupEnv("GOPATH")
	if !exists {
		fmt.Fprint(os.Stderr, "$GOPATH not set")
		os.Exit(1)
	}

	// find the package to mutest.
	var pkg string
	if len(os.Args) == 2 {
		pkg = os.Args[1]
	} else {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		if !strings.HasPrefix(wd, gopath) {
			fmt.Println("no package given and not in gopath")
			os.Exit(1)
		}
		// no need to use os.PathSeparator here because len(`/`) == len(`\`)
		pkg = wd[len(gopath)+len(`/src/`):]
	}
	return config{
		pkg:     pkg,
		gopath:  gopath,
		pkgFull: filepath.Join(gopath, "src", pkg),
	}
}

// sanityCheck verifies that the pkg we are trying to mutest compiles and that
// the tests pass.
func sanityCheck(cfg config) {
	{ // verify it compiles
		cmd := exec.Command("go", "build", cfg.pkg)
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to build package")
			os.Exit(1)
		}
		// remove any binary generated
		exec.Command("go", "clean").Run()
	}
	{ // verify tests pass
		cmd := exec.Command("go", "test", "-short", cfg.pkg)
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to test package")
			os.Exit(1)
		}
	}
	{ // verify that everything is already gofmt -s before
		finfos, err := ioutil.ReadDir(cfg.pkgFull)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}

		for _, finfo := range finfos {
			if !strings.HasSuffix(finfo.Name(), ".go") {
				continue
			}
			cmd := exec.Command("gofmt", "-s", "-d", filepath.Join(cfg.pkgFull, finfo.Name()))
			var b bytes.Buffer // need a buffer because gofmt doesn't return non-zero on diff
			cmd.Stdout = &b
			if err := cmd.Run(); err != nil || b.Len() > 0 {
				fmt.Printf("gofmt your package before running godzilla\n	gofmt -s -w %s\n", filepath.Join(cfg.pkgFull, "*go"))
				os.Exit(1)
			}
		}
	}
}

func generateCoverprofile(pkg string) []*cover.Profile {
	f, err := ioutil.TempFile("", "coverprofile")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	cmd := exec.Command("go", "test", "-short", "-coverprofile", f.Name(), pkg)
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	profiles, err := cover.ParseProfiles(f.Name())
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// remove all the Blocks that aren't covered
	for _, profile := range profiles {
		for i := 0; i < len(profile.Blocks); i++ {
			if profile.Blocks[i].Count == 0 {
				profile.Blocks = profile.Blocks[:i+copy(profile.Blocks[i:], profile.Blocks[i+1:])]
				i--
			}
		}
	}

	return profiles
}

func main() {
	cfg := getRunConfig()

	sanityCheck(cfg)

	coverprofiles := generateCoverprofile(cfg.pkg)

	// Create a temporary location to store all the mutated code
	tmpDir, err := ioutil.TempDir("", "godzilla")
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	results := make(chan result)
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		close(results)
	}()

	// build all the mutators
	mtrs := []mutators.Mutator{
		mutators.SwapIfElse,
		mutators.ConditionalsBoundaryMutator,
		mutators.MathMutator,
		mutators.MathAssignMutator,
		mutators.VoidCallRemoverMutator,
		mutators.BooleanOperatorsMutator,
		//mutators.ReturnValueMutator,
		//mutators.DebugInspect,
	}

	// build the "list" of mutators.
	c := make(chan mutators.Mutator, len(mtrs))
	for _, mutator := range mtrs {
		c <- mutator
	}
	close(c)

	// launch all mutator worker.
	var wg sync.WaitGroup
	for n := 0; n < runtime.NumCPU(); n++ {
		workdir := filepath.Join(tmpDir, "godzilla"+strconv.Itoa(n))
		if err := os.Mkdir(workdir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		w := worker{
			mutantDir:     workdir,
			originalDir:   cfg.pkgFull,
			results:       results,
			coverprofiles: coverprofiles,
		}
		wg.Add(1)
		go w.Mutate(c, &wg)
	}

	// once they're done close the results.
	go func() {
		wg.Wait()
		close(results)
	}()

	// aggregate the results.
	var res result
	for r := range results {
		res.alive += r.alive
		res.total += r.total
		res.skipped += r.skipped
	}

	fmt.Printf("score: %.2f%% (%d killed, %d alive, %d total, %d skipped)\n", float64(res.total-res.alive)/float64(res.total), res.total-res.alive, res.alive, res.total, res.skipped)
}

// result is the data passed to the aggregator to sum the total number of mutant
// executed and killed for a particular mutation.
type result struct {
	alive, total, skipped int
}

// worker is a type that works on a specific mutant folder and pulls mutators
// from a channel
type worker struct {
	// the directory of the mutated source.
	mutantDir string
	// a reference to the original source, this is so if a test reads from a
	// file in the package (like binary data) we don't break that.
	originalDir string

	results chan result

	coverprofiles []*cover.Profile
}

// visitor is a struct that runs a particular mutation case on the ast.Package.
type visitor struct {
	parseInfo mutators.ParseInfo
	mutator   mutators.Mutator
	tester    tester
}

// Mutate starts mutating the source, it gets the mutators from the given
// channel.
func (w worker) Mutate(c chan mutators.Mutator, wg *sync.WaitGroup) {
	defer wg.Done()

	// Parse the entire package
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, w.originalDir, nil, parser.ParseComments)
	if err != nil {
		// The code compiled, this should never happen
		panic(err)
	}

	// find the real package we want to mutate. because both {{.}} and
	// {{.}}_test can exist in the same folder and it's a valid go package.
	// However no more than 2 package can exist in the same folder.
	var pkg *ast.Package
	for _, p := range pkgs {
		if !strings.HasSuffix(p.Name, "_test") {
			pkg = p
		}
	}

	var files []*ast.File
	for _, file := range pkg.Files {
		files = append(files, file)
	}

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	conf := types.Config{Importer: importer.Default()}
	if _, err = conf.Check(pkg.Name, fset, files, info); err != nil {
		fmt.Fprintln(os.Stderr, err.Error)
		return
	}

	for m := range c {
		for name, file := range pkg.Files {
			// don't mutate test files.
			if strings.HasSuffix(name, "_test.go") {
				continue
			}

			// find the block we actually care about.
			var blocks []cover.ProfileBlock
			for _, p := range w.coverprofiles {
				if !strings.HasSuffix(name, p.FileName) {
					continue
				}
				blocks = p.Blocks
				break
			}

			v := &visitor{
				mutator: m,
				parseInfo: mutators.ParseInfo{
					FileSet:       fset,
					CoveredBlocks: blocks,
					TypesInfo:     info,
				},
				tester: tester{
					mutantDir:   w.mutantDir,
					originalDir: w.originalDir,
					astFile:     file,
					astFileName: name,
					fset:        fset,
				},
			}

			ast.Walk(v, file)
			w.results <- v.tester.result
		}
	}
}

type tester struct {
	// the directory that this mutant should test into.
	mutantDir string

	originalDir string

	// the packages, either len is 1 or 2, if it's 2 its because we have {{.}}
	// and {{.}}_test
	astFile     *ast.File
	astFileName string

	fset *token.FileSet

	result result
}

// Test take the current ast.Package, rewrites the source and test it.
func (t *tester) Test() {
	// rewrite all files in the mutant dir
	baseName := filepath.Base(t.astFileName)
	file, err := os.OpenFile(filepath.Join(t.mutantDir, baseName), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}
	if err = format.Node(file, t.fset, t.astFile); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

	// Verify that the mutant we generated actually compiles
	cmd := exec.Command("go", "build")
	cmd.Dir = t.mutantDir
	if err := cmd.Run(); err != nil {
		t.result.skipped++
		// that message is not expected to appear. That implies one of the
		// mutator build a code tree that doesn't compile. Ideally we could
		// report the code generated and why it didn't compile.
		fmt.Println("invalid build")
		return
	}

	// execute `go test` in that folder, the GOPATH can stay the same as the
	// callers.
	// BUG(hydroflame): when the test package is called *_test this will fail to
	// import the actual mutant, make the GOPATH var of the cmd be
	// `GOPATH=.../mutantDir:ActualGOPATH`
	cmd = exec.Command("go", "test", "-short")
	cmd.Dir = t.mutantDir
	t.result.total++
	if getExitCode(cmd.Run()) != 0 {
		// the tests failed, the mutant is killed.
		return
	}
	t.result.alive++

	// Print the diff of the old and new file to the user.
	cmd = exec.Command("diff", "-u",
		filepath.Join(t.originalDir, baseName),
		filepath.Join(t.mutantDir, baseName))
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// getExitCode returns the exit code of an error returned by os/exec.Cmd.Run()
// or zero if the error is nil.
func getExitCode(err error) int {
	if err == nil {
		return 0
	} else if e, ok := err.(*exec.ExitError); ok {
		return e.Sys().(syscall.WaitStatus).ExitStatus()
	}
	// shouldn't really ever happen but if it does say it's an error.
	return 1
}

// Visit simply forwards the node to the mutator func of the visitor. This
// function makes *visitor implement the ast.Visitor interface.
func (v *visitor) Visit(node ast.Node) ast.Visitor {
	if node == nil { // sometimes called with nil for some reason.
		return v
	}

	v.mutator(v.parseInfo, node, &v.tester)
	return v
}
