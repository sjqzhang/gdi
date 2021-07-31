package gdi

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var globalGDI *GDIPool

type GDIPool struct {
	creator       map[reflect.Type]interface{}
	creatorLocker sync.RWMutex
	typeToValues  map[reflect.Type]reflect.Value
	ttvLocker     sync.RWMutex
}

func init() {
	globalGDI = NewGDIPool()
}

func NewGDIPool() *GDIPool {

	return &GDIPool{
		creator:       make(map[reflect.Type]interface{}),
		creatorLocker: sync.RWMutex{},
		typeToValues:  make(map[reflect.Type]reflect.Value),
		ttvLocker:     sync.RWMutex{},
	}
}

func RegisterObject(funcObjOrPtrs ...interface{}) {

	globalGDI.RegisterObject(funcObjOrPtrs...)

}

func Get(t interface{}) (value interface{}) {
	return globalGDI.Get(t)
}

func Build() {
	globalGDI.Build()
}

func (gdi *GDIPool) RegisterObject(funcObjOrPtrs ...interface{}) {
	for i, _ := range funcObjOrPtrs {
		funcObjOrPtr := funcObjOrPtrs[i]
		ftype := reflect.TypeOf(funcObjOrPtr)
		if _, ok := gdi.get(ftype); ok {
			panic(fmt.Sprintf("double register %v", ftype))
		}
		if ftype.Kind() == reflect.Ptr { // 对指针对象做特殊处理
			gdi.set(ftype, funcObjOrPtr)
			continue
		}
		ftype, err := parsePoolFunc(funcObjOrPtr)
		if err != nil {
			panic(err)
		}
		gdi.set(ftype, funcObjOrPtr)
	}
}

func (gdi *GDIPool) Build() *GDIPool {
	for _, v := range gdi.typeToValues {
		gdi.build(v)
	}
	return gdi
}

func (gdi *GDIPool) all() map[reflect.Type]reflect.Value {
	objs:=make(map[reflect.Type]reflect.Value)
	gdi.ttvLocker.Lock()
	defer gdi.ttvLocker.Unlock()
	for k,v:=range gdi.typeToValues {
		objs[k]=v
	}
	return objs
}

func (gdi *GDIPool) build(v reflect.Value) {
	for i := 0; i < v.Elem().NumField(); i++ {
		if (v.Elem().Field(i).Kind() == reflect.Ptr || v.Elem().Field(i).Kind() == reflect.Interface) && v.Elem().Field(i).IsNil() {
			ftype := reflect.TypeOf(v.Elem().Field(i).Interface())
			if ftype == nil { //当为接口时ftype为空
				isExist := false
				for t, vTmp := range gdi.all() {
					if t.Implements(v.Elem().Field(i).Type()) {
						v.Elem().Field(i).Set(vTmp)
						isExist = true
						break
					}
				}
				if !isExist {
					panic(fmt.Sprintf("inject type %v not found,please Register first!!!!", v.Elem().Field(i).Type()))
				}
			} else {
				if value, ok := gdi.get(ftype); ok {
					v.Elem().Field(i).Set(value)
				} else {
					panic(fmt.Sprintf("inject type %v not found,please Register first!!!!", ftype))
				}
			}

		}

	}
}

func (gdi *GDIPool) Get(t interface{}) (value interface{}) {
	ftype := reflect.TypeOf(t)
	result, ok := gdi.get(ftype)
	if !ok {
		return nil
	}
	return result.Interface()

}

func (gdi *GDIPool) get(t reflect.Type) (result reflect.Value, ok bool) {
	gdi.ttvLocker.RLock()
	defer gdi.ttvLocker.RUnlock()
	result, ok = gdi.typeToValues[t]
	return
}


func create(fun interface{}) reflect.Value {
	values := reflect.ValueOf(fun).Call([]reflect.Value{})
	if len(values) == 0 {
		panic(fmt.Sprintf("Dependency injector: func return value must be a pointer or a pointer with error, %v", reflect.TypeOf(fun)))
	}
	if len(values) > 2 {
		panic(fmt.Sprintf("Dependency injector: func return value must be a pointer or a pointer with error, %v", reflect.TypeOf(fun)))
	}
	if len(values) == 2 && values[1].Interface() != nil {
		panic(fmt.Sprintf("init %v throw %v", reflect.TypeOf(fun), reflect.ValueOf(values[1]).Interface()))
	}
	return values[0]
}



func (gdi *GDIPool) set(outType reflect.Type, f interface{}) {
	if _,ok:=gdi.get(outType);ok {
		panic(fmt.Sprintf("double register %v", outType))
	}
	gdi.creatorLocker.Lock()
	defer gdi.creatorLocker.Unlock()
	if reflect.TypeOf(f).Kind() == reflect.Func && f != nil {
		gdi.creator[outType] = f
	}
	gdi.ttvLocker.Lock()
	defer gdi.ttvLocker.Unlock()
	if f != nil {
		if reflect.TypeOf(f).Kind() == reflect.Func {
			gdi.typeToValues[outType] = create(f)
		} else if reflect.TypeOf(f).Kind() == reflect.Ptr {
			gdi.typeToValues[outType] = reflect.ValueOf(f)
		} else {
			panic(fmt.Sprintf("%v type not support ", reflect.TypeOf(f)))
		}
	}
}


func parsePoolFunc(f interface{}) (outType reflect.Type, e error) {
	ftype := reflect.TypeOf(f)
	if ftype.Kind() != reflect.Func {
		e = errors.New("it's not a func")
		return
	}

	if ftype.NumOut() > 2 {
		e = errors.New("return values should be less 2 !!!!")
		return
	}
	outType = ftype.Out(0)
	if outType.Kind() != reflect.Ptr {
		e = errors.New("the first return value must be an object pointer")
		return
	}
	return
}
