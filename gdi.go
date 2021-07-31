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
	globalGDI = newGDIPool()
}

func newGDIPool() *GDIPool {

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
	v, ok := globalGDI.Get(t)
	if !ok {
		return nil
	}
	return v
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
		gdi.bind(ftype, funcObjOrPtr)
	}
}

func (gdi *GDIPool) Build() *GDIPool {
	gdi.ttvLocker.Lock()
	defer gdi.ttvLocker.Unlock()
	for k, v := range gdi.typeToValues {
		if v.Kind() == reflect.Ptr && v.IsNil() {
			obj, ok := gdi.getOrCreate(k)
			gdi.build(v)
			if !ok {
				panic(fmt.Sprintf("inject %v error", k.Kind()))
			}
			gdi.typeToValues[k] = obj
		} else {
			gdi.build(v)
		}
	}
	return gdi
}

func (gdi *GDIPool) build(v reflect.Value) {
	for i := 0; i < v.Elem().NumField(); i++ {
		if (v.Elem().Field(i).Kind() == reflect.Ptr || v.Elem().Field(i).Kind() == reflect.Interface) && v.Elem().Field(i).IsNil() {
			ftype := reflect.TypeOf(v.Elem().Field(i).Interface())
			if ftype == nil { //当为接口时ftype为空
				isExist := false
				for t, vTmp := range gdi.typeToValues {
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
				if value, ok := gdi.typeToValues[ftype]; ok {
					v.Elem().Field(i).Set(value)
				} else {
					panic(fmt.Sprintf("inject type %v not found,please Register first!!!!", ftype))
				}
			}

		}

	}
}

func (gdi *GDIPool) Get(t interface{}) (value interface{}, ok bool) {
	ftype := reflect.TypeOf(t)
	result, o := gdi.get(ftype)
	value = result.Interface()
	ok = o
	return

}

func (gdi *GDIPool) get(t reflect.Type) (result reflect.Value, ok bool) {
	gdi.ttvLocker.RLock()
	defer gdi.ttvLocker.RUnlock()
	result, ok = gdi.typeToValues[t]
	return
}

func (gdi *GDIPool) getFunc(t reflect.Type) (fun interface{}, ok bool) {
	gdi.creatorLocker.RLock()
	defer gdi.creatorLocker.RUnlock()
	fun, ok = gdi.creator[t]
	return
}

// get .
func (gdi *GDIPool) getOrCreate(t reflect.Type) (result reflect.Value, ok bool) {
	result, ok = gdi.get(t)
	if ok {
		return
	}

	fun, ok := gdi.getFunc(t)
	// 没有找到对应的方法不进行create
	if !ok {
		return
	}

	gdi.set(t, create(fun))

	//gdi.ttvLocker.Lock()
	//defer gdi.ttvLocker.Unlock()
	//gdi.typeToValues[t] = create(fun)

	return
}

func create(fun interface{}) reflect.Value {
	values := reflect.ValueOf(fun).Call([]reflect.Value{})
	if len(values) == 0 {
		throwPanic(fun)
	}
	if len(values) > 2 {
		throwPanic(fun)
	}
	if len(values) == 2 && values[1].Interface() != nil {
		panic(fmt.Sprintf("init %v throw %v", reflect.TypeOf(fun), reflect.ValueOf(values[1]).Interface()))
	}
	return values[0]
}

func throwPanic(fun interface{}) {
	errMsg := fmt.Sprintf("Dependency injector: func return value must be a pointer or a pointer with error, %v", reflect.TypeOf(fun))
	panic(errMsg)
}

func (gdi *GDIPool) set(outType reflect.Type, f interface{}) {
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

func (gdi *GDIPool) bind(outType reflect.Type, f interface{}) {

	if _, ok := gdi.get(outType); ok {
		panic(fmt.Sprintf("double register %v", outType))
	} else {
		gdi.set(outType, f)
	}

}

func (gdi *GDIPool) allType() (list []reflect.Type) {
	gdi.ttvLocker.RLock()
	defer gdi.ttvLocker.RUnlock()

	for t := range gdi.typeToValues {
		list = append(list, t)
	}
	return
}

func (gdi *GDIPool) di(dest interface{}, call func(reflect.Value)) {
	allFields(dest, call)
}

func fetchValue(dest, src interface{}) bool {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr {
		return false
	}
	value = value.Elem()
	srvValue := reflect.ValueOf(src)
	if value.Type() == srvValue.Type() {
		value.Set(srvValue)
		return true
	}
	return false
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

// allFields
func allFields(dest interface{}, call func(reflect.Value)) {
	destVal := indirect(reflect.ValueOf(dest))
	destType := destVal.Type()
	if destType.Kind() != reflect.Struct && destType.Kind() != reflect.Interface {
		return
	}

	for index := 0; index < destVal.NumField(); index++ {
		val := destVal.Field(index)
		call(val)
	}
}

// allFieldsFromValue
func allFieldsFromValue(val reflect.Value, call func(reflect.Value)) {
	destVal := indirect(val)
	destType := destVal.Type()
	if destType.Kind() != reflect.Struct && destType.Kind() != reflect.Interface {
		return
	}
	for index := 0; index < destVal.NumField(); index++ {
		val := destVal.Field(index)
		call(val)
	}
}

func indirect(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr || reflectValue.Kind() == reflect.Interface {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}
