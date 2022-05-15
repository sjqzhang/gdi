package gdi

import (
	"fmt"
	"testing"
)

func init() {
	ScanPkgPaths("github.com/sjqzhang/gdi")
}

type gdiTest struct {
	Name string
	f    *string `inject:"name:test"`
}

type gdiTest2 struct {
	Name  string
	Name2 *string `inject:"name:test"`
	I     igdiTest
}
type igdiTest interface {
	Add(a, b int) int
}

func (g *gdiTest) Add(a, b int) int {
	return a + b
}

func TestAll(t *testing.T) {

	name := "gdi"
	gp := NewGDIPool()
	gp.Debug(true)
	gp.ScanPkgPaths("github.com/sjqzhang/gdi")
	gp.IgnoreInterfaceInject(false)
	gp.Register(&gdiTest{
		Name: name,
	}, func() *gdiTest2 {
		return &gdiTest2{
			Name: name,
		}
	}, func() (*string, string) {
		var name string
		name = "jqzhang"
		return &name, "test"
	})
	gp.Init()
	var g *gdiTest
	var g2 *gdiTest2

	type Inner struct {
		g *gdiTest
		g2 *gdiTest2
	}

	var i Inner

	gp.DI(&i)





	if gp.Get(g).(*gdiTest).Name != name {
		t.Fail()
	}
	if gp.Get(g2).(*gdiTest2).Name != name {
		t.Fail()
	}
	if gp.Get(g2).(*gdiTest2).I.Add(1, 2) != 3 {
		t.Fail()
	}

	if *gp.Get(g2).(*gdiTest2).Name2 != "jqzhang" {
		t.Fail()
	}

}

func TestAll2(t *testing.T) {

	name := "gdi"

	IgnoreInterfaceInject(false)

	Register(&gdiTest{
		Name: name,
	}, func() *gdiTest2 {
		return &gdiTest2{
			Name: name,
		}
	}, func() (*string, string) {
		var name string
		name = "jqzhang"
		return &name, "test"
	})
	Init()
	var g *gdiTest
	var g2 *gdiTest2
	if Get(g).(*gdiTest).Name != name {
		t.Fail()
	}
	if Get(g2).(*gdiTest2).Name != name {
		t.Fail()
	}
	if Get(g2).(*gdiTest2).I.Add(1, 2) != 3 {
		t.Fail()
	}
	if *Get(g2).(*gdiTest2).Name2 != "jqzhang" {
		t.Fail()
	}

}

type Addr struct {
	Email string
	addr  string
}

type Person struct {
	addr *Addr
	Name string
}

type A struct {
	p *Person
}

func TestAll3(t *testing.T) {
	Debug(true)
	Register(&Person{Name: "hello world"})

	Init()

    var a A

	DI(&a)
	a.p.addr.Email="xxx"

	fmt.Println(a.p.addr.Email)




}
