package main

import (
	"fmt"
	"github.com/sjqzhang/gdi"

	//"github.com/sjqzhang/gdi/tl"
)

type AA struct {
	B *BB
	A *string // `inject:"name:hello"`
	*CC
	//a interface{} //`inject:"name:hello"`
	//e error

	//i IIer
}

type BB struct {
	D *DD
	C *CC
	e *EE
}

type CC struct {
	Name string
	Age  int
}

type DD struct {
	C *CC
	I IIer //`inject:"name:ii"` //注意当为接口时，这里不能是指针，且有多实现时，目前只能返回第一个实现
	E *EE
}

type cc struct {
	name string
	age *int
}

type EE struct {
	A *AA //`inject:"name:a" json:"a"`
	*FF
	*cc
}

type FF struct {
	Addr string
	T *TT //`inject:"name:ttt"`
	Hello *string // `inject:"name:hello"`
}

type TT struct {
	Hl string
}

type IIer interface {
	Add(a, b int) int
	Sub(a,b int) int

}

type II struct {
	A string //`inject:"name:hello"`
	Home string // `inject:"name:home"`
}

type Q struct {

	//a *AA
}

func (ii *Q) Add(a, b int) int {

	return a + b
}

func (ii *Q) Sub(a, b int) int {

	fmt.Println("Q sub")
	return a + b
}

type Z struct {

	//a *AA
}

func (ii *Z) Add(a, b int) int {

	return a + b
}

func (ii *Z) Sub(a, b int) int {

	return a + b
}




func (ii *II) Add(a, b int) int {
	fmt.Println("ii.a->",ii.A)
	fmt.Println("ii.home->",ii.Home)
	return a + b
}

func (d *DD) Say(hi string) string {

	fmt.Println(hi + ", welcome")

	return hi

}

func (d *DD) Add(a, b int) int { //注意：当有多个实现时，存在不确定因索

	return a - b

}

//func (d *CC) Add(a, b int) int { //注意：当有多个实现时，存在不确定因索
//
//	return a - b
//
//}

func init() {
	//gdi.Register(&CC{
	//	Name: "jq",
	//})

	//gdi.Register(
	//	&AA{},
	//	&BB{},
	//	func() *DD {
	//		return &DD{}
	//	},
	//) //可以一次注册多个对象
	////gdi.Register(&BB{})
	////gdi.Register(&DD{})
	////gdi.Register(&CC{})
	//gdi.Register(func() (*II,string){
	//	return &II{
	//
	//	},"ii"
	//}) //简单对象
	////gdi.Register(&FF{
	////	Addr: "SZ",
	////}) //简单对象
	//
	//gdi.Register(func() *CC { //复杂对象
	//
	//	age := func() int { //可进行复杂构造，这只是示例
	//		return 10 + 4
	//	}
	//	return &CC{
	//		Name: "hello world",
	//		Age:  age(),
	//	}
	//})
	//
	//gdi.Register(func() (*EE, error) { //带错误的注册
	//
	//	return &EE{}, nil
	//})
	//
	//gdi.Register(func() (*TT,string) {
	//
	//	return &TT{
	//		Hl: "aaaa",
	//	},"ttt"
	//})
	//
	//gdi.Register(func() (*string,string) {
	//
	//	var name string
	//	name="xsdasdfaf"
	//	return &name,"hello"
	//})

}

func main() {



	gdi.Debug(true)      //显示注入信息，方便排错，需在gdi.Init()方法之前调用
	gdi.AutoCreate(true) //开启自动注入
	gdi.Register(&AA{})
	gdi.MapToImplement(&AA{},&Q{})
	gdi.Init()           //使用前必须先调用，当出现无解注入对象时会panic,避免运行时出现空指针
	var a *AA
	a = gdi.Get(a).(*AA) //说明，这里可以直接进行类型转换，不会出现空指针，当出现空指针时，gdi.Init()就会panic

	fmt.Println(a.B.C.Name)
	fmt.Println(a.B.D.C.Age)
	fmt.Println(a.B.D.Say("zhangsan"))

	fmt.Println(a.B.D.I.Add(2, 3))
	fmt.Println(a.B.D.E.A.B.D.E.A.B.D.E.A.B.C.Age)
	fmt.Println(a.B.D.E.A.B.C.Name)
	fmt.Println(a.B.D.E.T.Hl)
	fmt.Println(*a.B.D.E.Hello=="")
	fmt.Println(a.CC.Name)
	fmt.Println(a.B.e.name)
	//fmt.Println(a.a.(*II).Home)



	//var a AA
	//
	//var q Q
	//
	//_=q
	//fmt.Println(q)
	//
	//var z Z
	//
	//_=z
	//fmt.Println(z)
	//
	//fmt.Println(gdi.MapToImplement(&AA{},&Q{}))
	//
	////gdi.AutoCreate(true)
	//
	//gdi.Register(&a)
	//
	//gdi.Init()
	//
	//fmt.Println(a.i.Sub(4,5))
	//
	//
	//b:=AA{}
	//_=b
	//
	//gdi.DI(&a)


	//for _,v:=range gdi.GetAllTypes() {
	//	if reflect.TypeOf(AA{})==v {
	//
	//		for i:=0;i<v.Elem().NumField();i++ {
	//			//gdi.SetStructUnExportedStrField( v.Field(i),"Tag","xxx")
	//			fmt.Println()
	//		}
	//		fmt.Println("xxx")
	//	}
	//}

	//fmt.Println(a.B.D.C.Name)



}
