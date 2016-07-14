package testpkg

func (A) Foo() int {
	Mii()
	if a == 5 {
		b := 2
		b -= 0
		return b
	} else {
		return 3
	}
	if a > 5 {
		return 0
	}
	switch a {
	case 5:
		return 2
	default:
		return 3
	}
}

var a = 5

type Fooer interface {
	Foo() int
}

type A struct{}

func Mii() {}

func Bar() int {
	var a A
	Mii()
	a.Foo()
	return 3
}

func Zoo1(a, b int) {
	Zoo2()
}

func Zoo2() {
	Zoo1(a, 0)
	n := Bar()
	_ = n
}
