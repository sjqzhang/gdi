package gdi

import (
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unsafe"
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

var consoleLog = log.New(os.Stdout, "[gdi] ", log.LstdFlags)

func init() {
	globalGDI = NewGDIPool()
}

func NewGDIPool() *GDIPool {

	return &GDIPool{
		debug:         true,
		autoCreate:    true,
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
	if autoCreate {
		globalGDI.Debug(autoCreate)
	}
	globalGDI.AutoCreate(autoCreate)
}

func Get(t interface{}) (value interface{}) {
	return globalGDI.Get(t)
}

func Invoke(t interface{}) (interface{}, error) {
	return globalGDI.Invoke(t)
}

func DI(pointer interface{}) error {
	return globalGDI.DI(pointer)
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
		outType, err := gdi.parsePoolFunc(funcObjOrPtr)
		if err != nil {
			gdi.panic(err.Error())
		}
		if ftype.Kind() == reflect.Func {
			if ftype.NumIn() == 0 {
				gdi.set(outType, funcObjOrPtr)
			} else {
				gdi.creator[outType] = funcObjOrPtr
			}
		}
	}
}

func (gdi *GDIPool) Init() *GDIPool {

	for k := 0; k < len(gdi.creator)*len(gdi.creator); k++ {
		i := len(gdi.creator)
		j := 0
		for outype, creator := range gdi.creator {
			if _, ok := gdi.get(outype); ok {
				j++
			} else {
				funcType := reflect.TypeOf(creator)
				if funcType.Kind() == reflect.Func {
					if funcType.NumIn() == 0 {
						continue
					}
					var args []reflect.Value
					for n := 0; n < funcType.NumIn(); n++ {

						//t:=reflect.TypeOf(funcType.In(n))
						if itype, ok := gdi.get(funcType.In(n)); ok {
							args = append(args, itype)
						} else {
							break
						}
					}
					if funcType.NumIn() == len(args) {
						values := reflect.ValueOf(creator).Call(args)
						if len(values) > 1 && values[1].Kind() == reflect.String {
							name := values[1].Interface()
							gdi.namesToValues[name.(string)] = values[0]

						}
						if len(values) > 1 && values[1].Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
							if values[1].Interface() != nil {
								gdi.panic(fmt.Sprintf("(ERROR)create %v fail %v", outype, funcType))
							}
							gdi.typeToValues[outype] = values[0]
						}
						if len(values) == 1 {
							gdi.typeToValues[outype] = values[0]
						}
						gdi.log(fmt.Sprintf("inject type %v over by %v success", outype, funcType))
					}
				}
			}
		}
		if i == j {
			break
		}
	}

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
		field := v.Elem().Field(i)
		if field.Kind() != reflect.Interface && field.Kind() != reflect.Ptr {
			continue
		}
		fieldName := v.Type().Elem().Field(i).Name
		readOnly := false
		if !field.CanSet() {
			field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
			readOnly = true
		}
		defer func() {
			if readOnly {
				gdi.warn(fmt.Sprintf("inject fieldName:%v of %v", fieldName, v.Type()))
			} else {
				gdi.log(fmt.Sprintf("inject fieldName:%v of %v", fieldName, v.Type()))
			}

		}()
		name, ok := gdi.getTagAttr(v.Type().Elem().Field(i), "name")
		if ok && name != "" {
			if value, ok := gdi.getByName(name); ok {
				field.Set(value)
				continue
			} else {
				gdi.panic(fmt.Sprintf("name:%v type:%v object not found", name, field.Type()))
			}
		}
		if field.Kind() == reflect.Interface {
			if im, err := gdi.getByInterface(field.Type()); err==nil {
				field.Set(im)
				continue
			} else {
				gdi.panic(err.Error())
			}
		}
		if im, ok := gdi.get(field.Type()); ok {
			field.Set(im)
		} else {
			if gdi.autoCreate {
				value := reflect.New(field.Type().Elem())
				field.Set(value)
				gdi.warn(fmt.Sprintf("autoCreate type:%v fieldName:%v of %v", field.Type(), fieldName, v.Type()))
				gdi.set(field.Type(), value.Interface())
				gdi.build(value)
			}
		}

	}
}

func GetPtrUnExportFiled(s interface{}, filed string) reflect.Value {
	v := reflect.ValueOf(s).Elem().FieldByName(filed)
	// 必须要调用 Elem()
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

func SetPtrUnExportFiled(s interface{}, filed string, val interface{}) error {
	v := GetPtrUnExportFiled(s, filed)
	rv := reflect.ValueOf(val)
	if v.Kind() != v.Kind() {
		return fmt.Errorf("invalid kind, expected kind: %v, got kind:%v", v.Kind(), rv.Kind())
	}
	v.Set(rv)
	return nil
}

func (gdi *GDIPool) Debug(isDebug bool) {
	gdi.debug = isDebug
}
func Debug(isDebug bool) {
	globalGDI.debug = isDebug
}

func (gdi *GDIPool) DI(pointer interface{}) error {
	defer func() {
		if err := recover(); err != nil {
			gdi.warn(err.(error).Error())
		}
	}()
	var result reflect.Value
	ftype := reflect.TypeOf(pointer)
	if ftype.Kind() != reflect.Ptr {
		return errors.New("(ERROR) pointer type require")
	}
	result = reflect.ValueOf(pointer)
	if result.IsNil() {
		return errors.New("(ERROR) pointer is null ")
	}
	gdi.build(result)
	return nil
}

func (gdi *GDIPool) Invoke(t interface{}) (interface{}, error) {
	ftype := reflect.TypeOf(t)
	funValue := reflect.ValueOf(t)
	if ftype.Kind() == reflect.Func {
		var args []reflect.Value
		for i := 0; i < ftype.NumIn(); i++ {
			if v, ok := gdi.get(ftype.In(i)); ok {
				args = append(args, v)
			} else {
				gdi.warn(fmt.Sprintf("type '%v' not register", ftype))
			}
		}
		if ftype.NumIn() != len(args) {
			return nil, errors.New("(WARNING) can't inject all parameters!!!")
		}
		values := funValue.Call(args)
		if len(values) == 0 {
			return nil, nil
		}
		if last := values[len(values)-1]; last.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			var er error
			if last.Interface() != nil {
				er = last.Interface().(error)
			}
			if err, _ := last.Interface().(error); err != nil {
				return nil, er
			}
			if len(values) == 1 {
				return nil, er
			}
			if len(values) >= 2 {
				return values[0].Interface(), er
			}
		}
	} else {
		return nil, errors.New("(ERROR) just support func ")
	}
	return nil, nil

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

func (gdi *GDIPool) getByInterface(i reflect.Type) (value reflect.Value, err error) {
	cnt := 0
	var values []reflect.Value
	for t, v := range gdi.all() {
		if t.Implements(i) {
			cnt++
			value = v
			values = append(values, v)
		}
	}
	if cnt == 1 {
		return value, nil
	}
	if cnt > 1 {
		var msgs []string
		for _, v := range values {
			msgs = append(msgs, fmt.Sprintf("%v", v.Type()))
		}
		msg := fmt.Sprintf("there is one more object impliment %v interface [%v].", i.Name(), strings.Join(msgs, ","))
		return reflect.Value{}, fmt.Errorf(msg)
	}
	return reflect.Value{}, fmt.Errorf("interface type:%v not found", i.Name())
}

func (gdi *GDIPool) getByName(name string) (result reflect.Value, ok bool) {
	gdi.ttvLocker.RLock()
	defer gdi.ttvLocker.RUnlock()
	result, ok = gdi.namesToValues[name]
	return
}

func (gdi *GDIPool) log(msg string) {
	if gdi.debug {
		colorGreen := "\033[1;32m"
		colorNormal := "\033[0m"
		consoleLog.Println(colorGreen + msg + colorNormal)
	}
}
func (gdi *GDIPool) warn(msg string) {
	colorYellow := "\033[1;33m"
	colorNormal := "\033[0m"
	consoleLog.Println(fmt.Sprintf("%v%v%v", colorYellow, "WARNNING: "+msg, colorNormal))
}

func (gdi *GDIPool) panic(msg string) {
	var buf [2 << 10]byte
	colorRed := "\033[1;31m"
	colorNormal := "\033[0m"
	consoleLog.Println(colorRed + "PANIC:  注意查看以下提示（WARNNING:Pay attention to the following tips）" + colorNormal)
	consoleLog.Println(string(buf[:runtime.Stack(buf[:], true)]))
	consoleLog.Fatal(colorRed + msg + colorNormal)

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
				name := vals[1].Interface().(string)
				gdi.namesToValues[name] = vals[0]
				gdi.log(fmt.Sprintf("register by name, name:%v type:%v success", name, vals[0].Type()))
				return
			} else if len(vals) == 2 && vals[1].Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				if vals[1].Interface() != nil {
					gdi.panic(fmt.Sprintf("(ERROR) create %v '%v'", outType, vals[1].Interface().(string)))
				}
				gdi.typeToValues[outType] = vals[0]
			}
			gdi.log(fmt.Sprintf("register by type, type:%v success", outType))
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
