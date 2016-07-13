package testpkg

import (
	"testing"
)

func TestFoo(t *testing.T) {
	var a A
	if a.Foo() != 2 {
		t.Error(a.Foo())
	}
}

func TestBar(t *testing.T)  {}
func TestZoo1(t *testing.T) {}
func TestZoo2(t *testing.T) {}
