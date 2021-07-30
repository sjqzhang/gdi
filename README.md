## golang 依赖注入包(Golang Dependency Injection) 简称：GDI
## 原理
通过golang 的 init方法进行对象注册，使用反射技术对为空的指针对象进行赋值，完成对象的组装。

## 注意事项
- 注册对象必须写在init方法中
- 对象的类型必须是指针类型
- 最后一定要调用 gdi.Build() 方法
- 只支持单例实例，且只按类型进行反射注入
- 只能能过指针参数获取对象
- 需要注入的对象必须是导出属性（大写字母开头）

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
}

func (d *DD) Say(hi string) string  {

	fmt.Println(hi+ ", welcome")

	return hi

}

func init() {

	gdi.RegisterObject(&AA{})//简单对象
	gdi.RegisterObject(&BB{})
	gdi.RegisterObject(&DD{})

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

}

```