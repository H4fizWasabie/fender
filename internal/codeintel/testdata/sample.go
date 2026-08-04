package sample

import "fmt"

type Foo struct{ X int }

func (f *Foo) Bar() int { return f.X }

func Run() {
	f := &Foo{X: 1}
	fmt.Println(f.Bar())
}
