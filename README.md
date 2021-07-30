## golang 依赖注入包(Golang Dependency Injection) 简称：GDI
## 原理
通过golang 的 init方法进行对象注册，使用反射技术对为空的指针对象进行赋值，完成对象的组装。

## 注意事项
- 注册对象必须写在init方法中
- 对象的类型必须是指针类型
- 最后一定要调用 gdi.Build() 方法

## 如何安装
`go get 

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
	C *CC
}

type CC struct {
	Name string
	Age int
}

func init()  {

	gdi.RegisterObject(func() *AA {
		return &AA{

		}
	})

	gdi.RegisterObject(func() *BB {
		return &BB {

		}
	})

	gdi.RegisterObject(func() *CC {
		return &CC {
			Name: "hello world",
		}
	})
}



func main() {


	a,_:=gdi.Get(&AA{})
	gdi.Build()
	fmt.Println(a.(*AA).B.C.Name)

}

```