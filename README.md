## golang 依赖注入包(Golang Dependency Injection) 简称：GDI

## 原理

通过golang 的 init方法进行对象注册或自动注册对象，使用反射技术对为空的指针对象进行赋值，完成对象的组装。

## 优点：
- 零侵入
- 免注册（自动发现与注册）
- 零配置
- 易排查（错误信息明确）
- 支持多种依赖注入方式
- 支持生成依赖图
- 支持注解swagger路由
- 支持注解中间件
- 支持自动扫描指定包并注册对象
- 支持动态获取go源码

## 注意事项

- 注册对象必须写在init方法中(或在main中调用`gdi.GenGDIRegisterFile(false)`自动生成注册依赖,注意需要进行二次编译)
- 对象的类型必须是指针类型(接口类型除外)
- 最后一定要调用 gdi.Init() 方法
- 只支持单例实例，且只按类型进行反射注入
- 只能能过指针参数获取对象
- 构建后的对可以直接进行类型转换,参阅示例

## 注册对象的几种方式

```golang
    gdi.GenGDIRegisterFile(false) //全自动注册（推荐）
	gdi.Register(
		&AA{},//直接实例化对象（方式一）
		&BB{},
		func() *DD {//使用函数实例化对象，返回对象的指针（方式二）
			return &DD{}
		},
        func() (*CC,error) {//使用函数实例化对象，返回对象的指针，入创建对象是否出错（方式三）
        return &CC{},nil
        },
        func() (*EE,string) {//使用函数实例化对象，返回对象的指针及名称，后面可以使用名称进行注入，参考自带例子（方式四）
        return &EE{},"ee"
        },
	) //可以一次注册多个对象

```

## 如何安装

`go get -u github.com/sjqzhang/gdi`

## 使用示例

```golang
package main

import (
	"fmt"
	"github.com/sjqzhang/gdi"
)

type AA struct {
	B *BB
	A *string `inject:"name:hello"`
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
	I IIer `inject:"name:ii"` //注意当为接口时，这里不能是指针，且有多实现时，目前只能返回第一个实现
	E *EE
}

type EE struct {
	A *AA //`inject:"name:a" json:"a"`
	*FF
}

type FF struct {
	Addr string
	T *TT `inject:"name:ttt"`
	Hello *string  `inject:"name:hello"`
}

type TT struct {
	Hl string
}

type IIer interface {
	Add(a, b int) int
}

type II struct {
	A string `inject:"name:hello"`
	Home string `inject:"name:home"`
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

func init() {

	gdi.Register(
		&AA{},
		&BB{},
		func() *DD {
			return &DD{}
		},
	) //可以一次注册多个对象
	//gdi.Register(&BB{})
	//gdi.Register(&DD{})
	//gdi.Register(&CC{})
	gdi.Register(func() (*II,string){
		return &II{

		},"ii"
	}) //简单对象
	//gdi.Register(&FF{
	//	Addr: "SZ",
	//}) //简单对象

	gdi.Register(func() *CC { //复杂对象

		age := func() int { //可进行复杂构造，这只是示例
			return 10 + 4
		}
		return &CC{
			Name: "hello world",
			Age:  age(),
		}
	})

	gdi.Register(func() (*EE, error) { //带错误的注册

		return &EE{}, nil
	})

	gdi.Register(func() (*TT,string) {

		return &TT{
			Hl: "aaaa",
		},"ttt"
	})

	gdi.Register(func() (*string,string) {

		var name string
		name="xsdasdfaf"
		return &name,"hello"
	})

}

func main() {
	gdi.Debug(true)      //显示注入信息，方便排错，需在gdi.Init()方法之前调用
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
	fmt.Println(*a.B.D.E.Hello)
	fmt.Println(*a.A)

}

```
