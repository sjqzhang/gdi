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
	typeToValues map[reflect.Type]reflect.Value
	ttvLocker    sync.RWMutex
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

func RegisterObject(funcObjOrPtr interface{})  {

	globalGDI.RegisterObject(funcObjOrPtr)

}

func Get(t interface{})  (value interface{}, ok bool) {
	return globalGDI.Get(t)
}

func (gdi *GDIPool) RegisterObject(funcObjOrPtr interface{}) {
	ftype := reflect.TypeOf(funcObjOrPtr)
	if ftype.Kind() == reflect.Ptr {
		if _, ok := gdi.get(ftype); ok {
			panic(fmt.Sprintf("object %v is multiple registration", funcObjOrPtr))
		}
		gdi.bind(ftype, funcObjOrPtr)
		return
	}
	ftype, err := parsePoolFunc(funcObjOrPtr)
	if err != nil {
		panic(err)
	}
	gdi.bind(ftype, funcObjOrPtr)
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
		if v.Elem().Field(i).Kind() == reflect.Ptr && v.Elem().Field(i).IsNil() {
			ftype := reflect.TypeOf(v.Elem().Field(i).Interface())
			if value, ok := gdi.typeToValues[ftype]; ok {
				v.Elem().Field(i).Set(value)
			} else {
				panic(fmt.Sprintf("inject type %v error,not found", ftype))
			}
		}
	}
}

func (gdi *GDIPool) Get(t interface{}) (value interface{}, ok bool) {
	ftype := reflect.TypeOf(t)
	gdi.ttvLocker.RLock()
	defer gdi.ttvLocker.RUnlock()
	result, ok := gdi.typeToValues[ftype]
	value = result.Interface()
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

	gdi.ttvLocker.Lock()
	defer gdi.ttvLocker.Unlock()
	gdi.typeToValues[t] = create(fun)

	return
}

func create(fun interface{}) reflect.Value {
	values := reflect.ValueOf(fun).Call([]reflect.Value{})
	if len(values) == 0 {
		throwPanic(fun)
	}
	return values[0]
}

func throwPanic(fun interface{}) {
	errMsg := fmt.Sprintf("Dependency injector: func return to empty, %v", reflect.TypeOf(fun))
	panic(errMsg)
}

func (gdi *GDIPool) bind(outType reflect.Type, f interface{}) {
	gdi.creatorLocker.Lock()
	defer gdi.creatorLocker.Unlock()
	gdi.creator[outType] = f

	gdi.ttvLocker.Lock()
	defer gdi.ttvLocker.Unlock()
	gdi.typeToValues[outType] = create(f)
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
	if ftype.NumOut() != 1 {
		e = errors.New("return must be one object pointer")
		return
	}
	outType = ftype.Out(0)
	if outType.Kind() != reflect.Ptr {
		e = errors.New("return must be an object pointer")
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
