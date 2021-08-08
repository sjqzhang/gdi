package gdi

import "testing"

type gdiTest struct {
	Name string
	f *string
}
type gdiTest2 struct {
	Name string
	Name2 *string `inject:"name:test"`
	I igdiTest
}
type igdiTest interface {
	Add(a,b int) int
}

func (g *gdiTest)Add( a,b int) int  {
	return a+b
}

func TestAll(t *testing.T) {



	name:="gdi"
	gp:=NewGDIPool()
	gp.Register(&gdiTest{
		Name: name,
	}, func() *gdiTest2 {
		return &gdiTest2{
			Name:name,
		}
	}, func()(*string,string) {
		var name string
		name="jqzhang"
		return &name,"test"
	})
	gp.Init()
	var g *gdiTest
	var g2 *gdiTest2
	if gp.Get(g).(*gdiTest).Name!=name {
		t.Fail()
	}
	if gp.Get(g2).(*gdiTest2).Name!=name {
		t.Fail()
	}
	if gp.Get(g2).(*gdiTest2).I.Add(1,2)!=3 {
		t.Fail()
	}

	if *gp.Get(g2).(*gdiTest2).Name2!="jqzhang"{
		t.Fail()
	}



}

func TestAll2(t *testing.T) {



	name:="gdi"

	Register(&gdiTest{
		Name: name,
	}, func() *gdiTest2 {
		return &gdiTest2{
			Name:name,
		}
	},func()(*string,string) {
		var name string
		name="jqzhang"
		return &name,"test"
	})
	Init()
	var g *gdiTest
	var g2 *gdiTest2
	if Get(g).(*gdiTest).Name!=name {
		t.Fail()
	}
	if Get(g2).(*gdiTest2).Name!=name {
		t.Fail()
	}
	if Get(g2).(*gdiTest2).I.Add(1,2)!=3 {
		t.Fail()
	}
	if *Get(g2).(*gdiTest2).Name2!="jqzhang"{
		t.Fail()
	}

}