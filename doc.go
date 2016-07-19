// Package godzilla aim to provide mutation testing for go packages.
//
// Mutation testing requires a bit more of your time than golint or go vet. It
// tries to modify your code slightly (like changing a < for <=) and expects
// your tests to notice the changes, by failing. There are of course useless
// changes like `!(byte(a) > 0)` => `byte(a) <= 0` that godzilla tries to avoid,
// but even then it might make a change that is equivalent to the original
// program. Consider
//	1 func Max(s []float64) float64 {
//	2 	var max float64 = -math.MaxFloat64
//	3 	for _, f := range s {
//	4 		if f > max {
//	5 			max = f
//	6 		}
//	7 	}
//	8 	return max
//	9 }
// If we modify line 4 to
//	4 		if f >= max {
// The overall behavior of the function would not change even though the
// expression
//	f > max
// is different than
//	f >= max
// These are called equivalent mutants and it is a well known undecidable
// problem in computer science.
//
// godzilla can be invoke with:
//	godzilla [package]
// It will first try to compile, test (with -short) and gofmt your
// package. If any of these 3 step fails godzilla will exit early. The gofmt
// part is to reduce white noise in the output.
//
// A serie of mutation is then executed over your codebase and after each
// change your tests are executed. If they fail that means you have successfully
// detected the change godzilla introduced. If the tests pass however it might
// be either because your tests are not testing the mutated statement properly
// (eg. ignoring return values) or godzilla created an equivalent mutant. Code
// that is not covered is not mutated. godzilla will try to detect equivalent
// mutant as best it can, however some will slip through the crack.
//
// Most of the output from godzilla is diff -u of the mutated file and the
// original file
//	--- a.go	2016-07-19 02:46:07.000000000 -0400
//	+++ a.go	2016-07-19 14:20:31.000000000 -0400
//	@@ -21,7 +21,7 @@
//	 	Moo()
//	 	Mee()
//	 	Maa()
//	-	if a == 5 {
//	+	if a != 5 {
//	 		b := 2
//	 		b -= 0
//	 		return b
// as well as the final mutation score of your package
//	score: 50.0% (9 killed, 9 alive, 18 total, 0 skipped)
package godzilla
