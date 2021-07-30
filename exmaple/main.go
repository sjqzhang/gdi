package main

import (
	"fmt"
	"github.com/sjqzhang/gdi"
)

type AA struct {
	B *BB
}

type BB struct {
	H *DD
	C *CC
}

type CC struct {
	Name string
	Age  int
}

type DD struct {
	C *CC
	I IIer  //注意当为接口时，这里不能是指针，且有多实现时，目前只能返回第一个实现
}

type IIer interface {
	Add(a,b int) int


}


type II struct {

}

func (ii *II)Add(a,b int) int {
	return a+b
}



func (d *DD) Say(hi string) string  {

	fmt.Println(hi+ ", welcome")

	return hi

}

func init() {

	gdi.RegisterObject(&AA{})//简单对象
	gdi.RegisterObject(&BB{})
	gdi.RegisterObject(&DD{})
	gdi.RegisterObject(&II{})

	gdi.RegisterObject(func() *CC {//复杂对象

		age:= func() int { //可进行复杂构造，这只是示例
			return 10+4
		}
		return &CC{
			Name: "hello world",
			Age:  age(),
		}
	})
}

func main() {
	gdi.Build()
	var a *AA
	if v := gdi.Get(a); v != nil {
		a = v.(*AA)
	}
	fmt.Println(a.B.C.Name)
	fmt.Println(a.B.H.C.Age)
	fmt.Println(a.B.H.Say("zhangsan"))

	fmt.Println(a.B.H.I.Add(2,3))

}
