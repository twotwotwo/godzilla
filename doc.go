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
// that is not covered is not mutated.
package godzilla
