package testpkg

const bazoo = 3.0

func (A) Foo() int {
	_ = bazoo
	Myy()
	Mii()
	Moo()
	Mee()
	Maa()
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

func Myy() {}

func Mii() A {
	return A{}
}

func Moo() *A {
	var a A
	return &a
}

func Mee() A {
	var a A
	return a
}

func Maa() *A {
	var a *A
	return a
}

func Bar() (int, int) {
	var a A
	Mii()
	Myy()
	a.Foo()
	return 0, 1
}

func Zoo1(a, b int) {
	Zoo2()
}

func Zoo2() {
	Zoo1(a, 0)
	n, m := Bar()
	_, _ = n, m
}
