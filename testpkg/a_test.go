package testpkg

import (
	"testing"
)

func TestFoo(t *testing.T) {
	var a A
	a.Foo()
	NoUseless()
}

func TestBar(t *testing.T) {
	Bar()
}
func TestZoo1(t *testing.T) {}
func TestZoo2(t *testing.T) {}
