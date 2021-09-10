package gdi

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

var globalGDI *GDIPool

type GDIPool struct {
	debug         bool
	autoCreate    bool
	creator       map[reflect.Type]interface{}
	creatorLocker sync.RWMutex
	typeToValues  map[reflect.Type]reflect.Value
	namesToValues map[string]reflect.Value
	ttvLocker     sync.RWMutex
}

func init() {
	globalGDI = NewGDIPool()
}

func NewGDIPool() *GDIPool {

	return &GDIPool{
		debug:         false,
		autoCreate:    false,
		creator:       make(map[reflect.Type]interface{}),
		creatorLocker: sync.RWMutex{},
		typeToValues:  make(map[reflect.Type]reflect.Value),
		namesToValues: make(map[string]reflect.Value),
		ttvLocker:     sync.RWMutex{},
	}
}

func Register(funcObjOrPtrs ...interface{}) {

	globalGDI.Register(funcObjOrPtrs...)

}

func AutoCreate(autoCreate bool) {
	globalGDI.AutoCreate(autoCreate)
}

func Get(t interface{}) (value interface{}) {
	return globalGDI.Get(t)
}

func New(t interface{}) (value interface{}, err error) {
	return globalGDI.New(t)
}

func GetWithCheck(t interface{}) (value interface{}, ok bool) {
	return globalGDI.GetWithCheck(value)
}

func Init() {
	globalGDI.Init()
}

func (gdi *GDIPool) AutoCreate(autoCreate bool) {
	gdi.autoCreate = autoCreate
}

func (gdi *GDIPool) Register(funcObjOrPtrs ...interface{}) {
	for i, _ := range funcObjOrPtrs {
		funcObjOrPtr := funcObjOrPtrs[i]
		ftype := reflect.TypeOf(funcObjOrPtr)
		if ftype.Kind() != reflect.Ptr && ftype.Kind() != reflect.Func {
			gdi.warn(fmt.Sprintf("(WARNNING) register %v fail just support a struct pointer or a function return a struct pointer ", ftype))
			return
		}
		if _, ok := gdi.get(ftype); ok {
			gdi.panic(fmt.Sprintf("double register %v", ftype))
			return
		}
		if ftype.Kind() == reflect.Ptr && ftype.Elem().Kind() == reflect.Struct { // 对指针对象做特殊处理
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
	for _, v := range gdi.namesToValues {
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
	if v.Elem().Kind() != reflect.Struct {
		return
	}
	for i := 0; i < v.Elem().NumField(); i++ {
		if (v.Elem().Field(i).Kind() == reflect.Ptr || v.Elem().Field(i).Kind() == reflect.Interface) && v.Elem().Field(i).IsNil() && v.Elem().Field(i).CanSet() {
			ftype := reflect.TypeOf(v.Elem().Field(i).Interface())
			fieldName := reflect.TypeOf(v.Elem().Interface()).Field(i).Name
			fieldType := reflect.TypeOf(v.Elem().Interface()).Field(i).Type
			if ftype == nil { //当为接口时ftype为空
				name, ok := gdi.getTagAttr(reflect.TypeOf(v.Elem().Interface()).Field(i), "name")
				if ok {
					if value, ok := gdi.namesToValues[name]; ok {
						v.Elem().Field(i).Set(value)
						gdi.log(fmt.Sprintf("inject by name the field %v of %v by %v", fieldName, v.Type(), fieldType))
						continue
					}
				}
				isExist := false
				for t, vTmp := range gdi.all() {
					if t.Implements(v.Elem().Field(i).Type()) {
						v.Elem().Field(i).Set(vTmp)
						gdi.log(fmt.Sprintf("inject interface by type the field %v of %v by %v", fieldName, v.Type(), fieldType))
						isExist = true
						break
					}
				}
				if !isExist {
					gdi.panic(fmt.Sprintf("inject type %v not found,please Register first!!!!", v.Elem().Field(i).Type()))
				}
			} else {
				name, ok := gdi.getTagAttr(reflect.TypeOf(v.Elem().Interface()).Field(i), "name")
				if ok {
					if value, ok := gdi.namesToValues[name]; ok {
						v.Elem().Field(i).Set(value)
						gdi.log(fmt.Sprintf("inject by name the field %v of %v by %v", fieldName, v.Type(), fieldType))
						continue
					} else {
						gdi.panic(fmt.Sprintf("the name of '%v' not found,please register first", name))
					}
				}
				if ftype.Elem().Kind() != reflect.Struct {
					gdi.warn(fmt.Sprintf("(WARNNING) inject %v ignore of %v,type just support Struct ", ftype, v.Type()))
					continue
				}
				if value, ok := gdi.get(ftype); ok {
					v.Elem().Field(i).Set(value)
					gdi.log(fmt.Sprintf("inject by type the field %v of %v by %v", fieldName, v.Type(), fieldType))
					continue
				} else {
					if gdi.autoCreate {
						value = reflect.New(ftype.Elem())
						v.Elem().Field(i).Set(value)
						gdi.set(ftype, value.Interface()) //must understand the reflect type and reflect value and interface{} relation
						gdi.build(value)
						gdi.log(fmt.Sprintf("autocreate %v inject by type the field %v of %v by %v", ftype, fieldName, v.Type(), fieldType))
					} else {
						gdi.panic(fmt.Sprintf("inject type %v not found,please Register first!!!!", ftype))
					}
				}
			}
		} else if !v.Elem().Field(i).CanSet() && (v.Elem().Field(i).Kind() == reflect.Ptr || v.Elem().Field(i).Kind() == reflect.Interface) && v.Elem().Field(i).IsNil() {

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

func (gdi *GDIPool) New(t interface{}) (value interface{}, err error) {
	var result reflect.Value
	ftype := reflect.TypeOf(t)
	if ftype.Kind() != reflect.Ptr {
		return nil, errors.New("(ERROR) pointer type require")
	}
	result = reflect.New(ftype.Elem())
	gdi.build(result)
	return result.Interface(), nil
}

func (gdi *GDIPool) Get(t interface{}) (value interface{}) {
	var ok bool
	var result reflect.Value
	if name, o := t.(string); o {
		result, ok = gdi.getByName(name)
	} else {
		ftype := reflect.TypeOf(t)
		result, ok = gdi.get(ftype)
	}
	if !ok {
		gdi.warn(fmt.Sprintf("can't found %v,Is gdi.Init() called?", t))
		return nil
	}
	return result.Interface()

}
func (gdi *GDIPool) GetWithCheck(t interface{}) (value interface{}, ok bool) {
	var result reflect.Value
	if name, o := t.(string); o {
		result, ok = gdi.getByName(name)
	} else {
		ftype := reflect.TypeOf(t)
		result, ok = gdi.get(ftype)
	}
	if !ok {
		return nil, false
	}
	value = result.Interface()
	return

}

func (gdi *GDIPool) getTagAttr(f reflect.StructField, tagAttr string) (string, bool) {

	if tag, ok := f.Tag.Lookup("inject"); ok {
		m := make(map[string]string)
		tags := strings.Split(tag, ";")
		for _, t := range tags {
			kvs := strings.Split(t, ":")
			if len(kvs) == 1 {
				m[kvs[0]] = ""
			}
			if len(kvs) == 2 {
				m[kvs[0]] = kvs[1]
			}
		}
		v, o := m[tagAttr]
		return v, o
	}
	return "", false
}

func (gdi *GDIPool) get(t reflect.Type) (result reflect.Value, ok bool) {
	gdi.ttvLocker.RLock()
	defer gdi.ttvLocker.RUnlock()
	result, ok = gdi.typeToValues[t]
	return
}
func (gdi *GDIPool) getByName(name string) (result reflect.Value, ok bool) {
	gdi.ttvLocker.RLock()
	defer gdi.ttvLocker.RUnlock()
	result, ok = gdi.namesToValues[name]
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
	var buf [2 << 10]byte
	log.Println("PANIC:  注意查看以下提示（WARNNING:Pay attention to the following tips）")
	log.Println(string(buf[:runtime.Stack(buf[:], true)]))
	log.Fatal(msg)

}

func (gdi *GDIPool) create(fun interface{}) []reflect.Value {
	values := reflect.ValueOf(fun).Call([]reflect.Value{})
	if len(values) == 0 {
		gdi.panic(fmt.Sprintf("Dependency injector: func return value must be a pointer or a pointer with error, %v", reflect.TypeOf(fun)))

	}
	if len(values) > 2 {
		gdi.panic(fmt.Sprintf("Dependency injector: func return value must be a pointer or a pointer with error, %v", reflect.TypeOf(fun)))
	}

	if len(values) == 2 && values[1].Interface() != nil && reflect.TypeOf(values[1].Interface()).Kind() == reflect.Ptr {
		gdi.panic(fmt.Sprintf("init %v throw %v", reflect.TypeOf(fun), reflect.ValueOf(values[1]).Interface()))
	}
	//if len(values) == 2 && values[1].Interface()!=nil  && reflect.TypeOf( values[1].Interface()).Kind()==reflect.String {
	//	gdi.panic(fmt.Sprintf("init %v throw %v", reflect.TypeOf(fun), reflect.ValueOf(values[1]).Interface()))
	//}
	return values
}

func (gdi *GDIPool) set(outType reflect.Type, f interface{}) {

	if f != nil {
		if reflect.TypeOf(f).Kind() == reflect.Func {
			vals := gdi.create(f)
			if len(vals) == 1 {
				gdi.typeToValues[outType] = vals[0]
			} else if len(vals) == 2 && vals[1].Kind() == reflect.String {
				if _, ok := gdi.namesToValues[vals[1].Interface().(string)]; ok {
					gdi.panic(fmt.Sprintf("double register name: '%v'", vals[1].Interface().(string)))
				}
				gdi.namesToValues[vals[1].Interface().(string)] = vals[0]
			}
			gdi.log(fmt.Sprintf("inject %v success", outType))
		} else if reflect.TypeOf(f).Kind() == reflect.Ptr {
			gdi.typeToValues[outType] = reflect.ValueOf(f)
			//gdi.log(fmt.Sprintf("inject %v success", outType))
		} else {
			//gdi.typeToValues[outType] = reflect.ValueOf(f)
			gdi.panic(fmt.Sprintf("%v type not support ", reflect.TypeOf(f)))
		}
	} else {
		if _, ok := gdi.get(outType); ok {
			gdi.panic(fmt.Sprintf("double register %v", outType))
			return
		}
		gdi.creatorLocker.Lock()
		defer gdi.creatorLocker.Unlock()
		if reflect.TypeOf(f).Kind() == reflect.Func && f != nil {
			gdi.creator[outType] = f
		}
		gdi.ttvLocker.Lock()
		defer gdi.ttvLocker.Unlock()
	}
}

func (gdi *GDIPool) parsePoolFunc(f interface{}) (outType reflect.Type, e error) {
	ftype := reflect.TypeOf(f)
	if ftype.Kind() != reflect.Func {
		e = errors.New(fmt.Sprintf("%v it's not a func", f))
		return
	}
	if ftype.NumOut() == 0 {
		e = errors.New(fmt.Sprintf("%v return values should be a pointer", f))
		return
	}
	if ftype.NumOut() > 2 {
		e = errors.New("return values should be less 2 !!!!")
		return
	}
	outType = ftype.Out(0)
	if outType.Kind() != reflect.Ptr && outType.Kind() != reflect.Interface {
		e = errors.New("the first return value must be an object pointer")
		return
	}
	return
}
