package testpkg

var a = 5

func Foo() int {
	if a == 5 {
		return 2
	} else {
		return 3
	}
	switch a {
	case 5:
		return 2
	case 5:
		return 4
	default:
		return 3
	}
}
