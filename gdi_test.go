package gdi

import (
	"fmt"
	"regexp"
	"sync"
	"testing"
)

func init() {

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
		g  *gdiTest
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

	c := Class{
		Student: &Student{
			Name: "jqzhang",
			Age:  20,
		},
	}

	var s BQ

	_ = s

	Register(&c)
	Init()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		DIForTest(&s)
		wg.Done()
	}()
	go DIForTest(&s)
	wg.Wait()

	if s.Student.Name != c.Student.Name {
		t.Fail()
	}

}

func TestGetAllPackages(t *testing.T) {

	//getAllPackages()
	//genDependency()

	if genDependency() == "" {
		t.Fail()
	}

	src := `
func GenGDIRegisterFile(override bool) {
	globalGDI.GenGDIRegisterFile(override)
}

func getCurrentAbPathByCaller(skip int) string {
	var abPath string
	_, filename, _, ok := runtime.Caller(skip)
	if ok {
		abPath = path.Dir(filename)
	}
	return abPath
}

func (gdi *GDIPool) GenGDIRegisterFile(override bool) {
	fn := getCurrentAbPathByCaller(3) + "/gdi_gen.go"
	source := genDependency()
	if _, err := os.Stat(fn); err != nil {
		ioutil.WriteFile(fn, []byte(source), 0755)
	} else {
		if override {
			ioutil.WriteFile(fn, []byte(source), 0755)
		}
	}
	runCmd("gofmt","-w",fn)


}

    // Builder
    type PaginationBuilder struct {
        reorder      *Reorder
        pageSize     int
        page         int
        total        int
        selectColumn []string
    }

    // TotalPage .
    func (p *PaginationBuilder) Total() int {
        return p.total
    }

    func (p *PaginationBuilder) Order() interface{} {
        if p.reorder != nil {
            return p.Order()
        }
        return ""
    }

func CrudTemplate() string {

	return 
	// Code generated by 'dms new-po'
	package po
	{{.Import}}
	{{.Content}}

	// TakeChanges .
	func (obj *{{.Name}})TakeChanges(ctx context.Context) map[string]interface{} {
		if obj.changes == nil {
		return nil
	}
		result := make(map[string]interface{})
		for k, v := range obj.changes {
		result[k] = v
	}
	{{.SetMTime}}
		obj.changes = nil
		return result
	}

	{{range .Fields}}
	// Set{{.Value}} .
	func (obj *{{.StructName}}) Set{{.Value}} (ctx context.Context, {{.Arg}} {{.Type}}) {
		if obj.changes == nil {
			obj.changes = make(map[string]interface{})
		}
		obj.{{.Value}} = {{.Arg}}
		obj.changes["{{.Column}}"] = {{.Arg}}
	}
	{{ end }}
	
}


`
	i := 0
	for {
		old := len(src)

		reg := regexp.MustCompile("{[^{|}]*}|`[^`]+?`")

		src = reg.ReplaceAllString(src, "")
		if len(src) == old {
			break
		}
		i++
	}

	fmt.Println(src, i)
}

//type UserController struct {
//}
//
//func (u *UserController) Get(c *gin.Context) {
//	c.JSON(200, gin.H{
//		"message": "pong",
//	})
//}
//
//func TestGin(t *testing.T) {
//	s:=UserController{}
//	fmt.Sprintf("%s",s)
//	objs,err:= AutoRegisterByPackName("gdi.*Controller")
//	if err!=nil {
//		t.Fail()
//	}
//	//通过反射绑定路由
//	r:=gin.Default()
//	for _,o:=range objs {
//		if o.NumMethod()==0 {
//			continue
//		}
//		if !o.Type().Method(0).IsExported() {
//			continue
//		}
//		name:=o.Type().Method(0).Name
//
//		x:=o.Method(0).Interface()
//
//		switch x.(type) {
//		case func(*gin.Context):
//			r.Handle("GET","/"+name,x.(func(*gin.Context)))
//
//		}
//		fmt.Println(o,x)
//
//	}
//	r.Run() // listen and serve on
//
//}

func TestXxx(t *testing.T) {
	name := "HelloWorld"
	if ConvertToCamelCase(ConvertToSnakeCase(name)) != name {
		t.Fail()
	}
}
