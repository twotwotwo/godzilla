package testpkg

func (A) Foo() int {
	if a == 5 {
		b := 2
		return b
	} else {
		return 3
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

func Bar() {
	var a A
	a.Foo()
}

func Zoo1() {
	Zoo2()
}

func Zoo2() {
	Zoo1()
}
