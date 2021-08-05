package main

import (
	"fmt"
	"github.com/sjqzhang/gdi"
)

type AA struct {
	B *BB
}

type BB struct {
	D *DD
	C *CC
}

type CC struct {
	Name string
	Age  int
}

type DD struct {
	C *CC
	I IIer //注意当为接口时，这里不能是指针，且有多实现时，目前只能返回第一个实现
	E *EE
}

type EE struct {
	A *AA
	*FF
}

type FF struct {
	Addr string
}

type IIer interface {
	Add(a, b int) int
}

type II struct {
}

func (ii *II) Add(a, b int) int {
	return a + b
}

func (d *DD) Say(hi string) string {

	fmt.Println(hi + ", welcome")

	return hi

}

func (d *DD) Add(a, b int) int { //注意：当有多个实现时，存在不确定因索

	return a - b

}

func init() {

	gdi.RegisterObject(
		&AA{},
		&BB{},
		func() *DD {
			return &DD{}
		},
	) //可以一次注册多个对象
	//gdi.RegisterObject(&BB{})
	//gdi.RegisterObject(&DD{})
	//gdi.RegisterObject(&CC{})
	gdi.RegisterObject(&II{}) //简单对象
	//gdi.RegisterObject(&FF{
	//	Addr: "SZ",
	//}) //简单对象

	gdi.RegisterObject(func() *CC { //复杂对象

		age := func() int { //可进行复杂构造，这只是示例
			return 10 + 4
		}
		return &CC{
			Name: "hello world",
			Age:  age(),
		}
	})

	//gdi.RegisterObject(func() (*EE, error) { //带错误的注册
	//
	//	return &EE{}, nil
	//})

}

func main() {
	gdi.Debug(true)      //显示注入信息，方便排错，需在gdi.Init()方法之前调用
	gdi.AutoCreate(true) //开启自动注入
	gdi.Init()           //使用前必须先调用，当出现无解注入对象时会panic,避免运行时出现空指针
	var a *AA
	a = gdi.Get(a).(*AA) //说明，这里可以直接进行类型转换，不会出现空指针，当出现空指针时，gdi.Init()就会panic
	fmt.Println(a.B.C.Name)
	fmt.Println(a.B.D.C.Age)
	fmt.Println(a.B.D.Say("zhangsan"))

	fmt.Println(a.B.D.I.Add(2, 3))
	fmt.Println(a.B.D.E.A.B.D.E.A.B.D.E.A.B.C.Age)
	fmt.Println(a.B.D.E.A.B.C.Name)
	fmt.Println(a.B.D.E.Addr)

}
