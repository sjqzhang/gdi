package gdi

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
)

var globalGDI *GDIPool

type GDIPool struct {
	debug         bool
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
		debug:         false,
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

func Init() {
	globalGDI.Init()
}

func (gdi *GDIPool) RegisterObject(funcObjOrPtrs ...interface{}) {
	for i, _ := range funcObjOrPtrs {
		funcObjOrPtr := funcObjOrPtrs[i]
		ftype := reflect.TypeOf(funcObjOrPtr)
		if _, ok := gdi.get(ftype); ok {
			gdi.panic(fmt.Sprintf("double register %v", ftype))
		}
		if ftype.Kind() == reflect.Ptr { // 对指针对象做特殊处理
			gdi.set(ftype, funcObjOrPtr)
			continue
		}
		ftype, err := gdi.parsePoolFunc(funcObjOrPtr)
		if err != nil {
			gdi.panic(err.Error())
		}
		gdi.set(ftype, funcObjOrPtr)
	}
}

func (gdi *GDIPool) Init() *GDIPool {
	for _, v := range gdi.typeToValues {
		gdi.build(v)
	}
	return gdi
}

func (gdi *GDIPool) all() map[reflect.Type]reflect.Value {
	objs := make(map[reflect.Type]reflect.Value)
	gdi.ttvLocker.Lock()
	defer gdi.ttvLocker.Unlock()
	for k, v := range gdi.typeToValues {
		objs[k] = v
	}
	return objs
}

func (gdi *GDIPool) build(v reflect.Value) {
	for i := 0; i < v.Elem().NumField(); i++ {
		if (v.Elem().Field(i).Kind() == reflect.Ptr || v.Elem().Field(i).Kind() == reflect.Interface) && v.Elem().Field(i).IsNil() && v.Elem().Field(i).CanSet() {
			ftype := reflect.TypeOf(v.Elem().Field(i).Interface())
			if ftype == nil { //当为接口时ftype为空
				isExist := false
				for t, vTmp := range gdi.all() {
					if t.Implements(v.Elem().Field(i).Type()) {
						v.Elem().Field(i).Set(vTmp)
						gdi.log(fmt.Sprintf("interface %v injected by %v success", v.Elem().Field(i).Type(), t))
						isExist = true
						break
					}
				}
				if !isExist {
					gdi.panic(fmt.Sprintf("inject type %v not found,please Register first!!!!", v.Elem().Field(i).Type()))
				}
			} else {
				if value, ok := gdi.get(ftype); ok {
					v.Elem().Field(i).Set(value)
					gdi.log(fmt.Sprintf("pointer %v injected by %v success", v.Elem().Field(i).Type(), ftype))
				} else {
					gdi.panic(fmt.Sprintf("inject type %v not found,please Register first!!!!", ftype))
				}
			}
		} else if !v.Elem().Field(i).CanSet() && (v.Elem().Field(i).Kind() == reflect.Ptr || v.Elem().Field(i).Kind() == reflect.Interface) {
			gdi.warn(fmt.Sprintf("pointer %v injected by %v fail,because field not export", v.Elem().Field(i).Type(), reflect.TypeOf(v.Elem().Field(i))))
		}

	}
}

func (gdi *GDIPool) Debug(isDebug bool) {
	gdi.debug = isDebug
}
func Debug(isDebug bool) {
	globalGDI.debug = isDebug
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

func (gdi *GDIPool) log(msg string) {
	if gdi.debug {
		log.Println(msg)
	}
}
func (gdi *GDIPool) warn(msg string) {
	log.Println("WARNNING: " + msg)
}

func (gdi *GDIPool) panic(msg string) {
	log.Println("PANIC:  注意查看以下提示（WARNNING:Pay attention to the following tips）")
	log.Fatal(msg)
}

func (gdi *GDIPool) create(fun interface{}) reflect.Value {
	values := reflect.ValueOf(fun).Call([]reflect.Value{})
	if len(values) == 0 {
		gdi.panic(fmt.Sprintf("Dependency injector: func return value must be a pointer or a pointer with error, %v", reflect.TypeOf(fun)))
	}
	if len(values) > 2 {
		gdi.panic(fmt.Sprintf("Dependency injector: func return value must be a pointer or a pointer with error, %v", reflect.TypeOf(fun)))
	}
	if len(values) == 2 && values[1].Interface() != nil {
		gdi.panic(fmt.Sprintf("init %v throw %v", reflect.TypeOf(fun), reflect.ValueOf(values[1]).Interface()))
	}
	return values[0]
}

func (gdi *GDIPool) set(outType reflect.Type, f interface{}) {
	if _, ok := gdi.get(outType); ok {
		gdi.panic(fmt.Sprintf("double register %v", outType))
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
			gdi.typeToValues[outType] = gdi.create(f)
			gdi.log(fmt.Sprintf("inject %v success", outType))
		} else if reflect.TypeOf(f).Kind() == reflect.Ptr {
			gdi.typeToValues[outType] = reflect.ValueOf(f)
			gdi.log(fmt.Sprintf("inject %v success", outType))
		} else {
			gdi.panic(fmt.Sprintf("%v type not support ", reflect.TypeOf(f)))
		}
	}
}

func (gdi *GDIPool) parsePoolFunc(f interface{}) (outType reflect.Type, e error) {
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
