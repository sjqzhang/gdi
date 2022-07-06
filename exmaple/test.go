package main

import (
	"fmt"
	"github.com/sjqzhang/gdi"
)

type Student struct {
	Name string
	Age  int
}

type Class struct {
	Student *Student
}

type BQ struct {
	Student *Student
}


func main() {

	c := Class{
		Student: &Student{
			Name: "jqzhang",
			Age: 20,
		},
	}

	var s BQ

	_=s

	gdi.Register(&c)
	gdi.Init()

	gdi.DIForTest(&s)

	fmt.Println(s.Student)

}
