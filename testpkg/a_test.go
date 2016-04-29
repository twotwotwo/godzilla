package testpkg

import (
	"testing"
)

func TestFoo(t *testing.T) {
	if Foo() != 2 {
		t.Error(Foo())
	}
}
