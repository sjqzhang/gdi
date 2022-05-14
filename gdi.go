package gdi

import (
	"errors"
	"fmt"
	"log"
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
					gdi.panic(fmt.Sprintf("inject the field '%v' of '%v' fail,  not found,please Register first!!!!", v.Elem().Type().Field(i).Name, v.Type().Name()))
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
						gdi.panic(fmt.Sprintf("the field '%v' of '%v' inject faild,please Register first!!!! ", v.Elem().Type().Field(i).Name, v.Elem().Type().String()))
					}
				}
			}
		} else if !v.Elem().Field(i).CanSet() && (v.Elem().Field(i).Kind() == reflect.Ptr || v.Elem().Field(i).Kind() == reflect.Interface) && v.Elem().Field(i).IsNil() {

			setPrivateField := func(v reflect.Value, i int) {//not export fields
				defer func() {
					if err := recover(); err != nil {
						gdi.panic("(WARNNING) setPrivateField" + err.(error).Error())
					}
				}()
				if !v.Elem().Field(i).CanSet() { //私有变量
					rf := v.Elem().Field(i)
					if rf.Kind() == reflect.Interface {
						return
					}
					rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
					fieldName:=v.Elem().Type().Field(i).Name
					ftype:=v.Elem().Type().Field(i).Type
					if value, ok := gdi.get(rf.Type()); ok {
						rf.Set(value)
					} else {
						if gdi.autoCreate {
							value = reflect.New(rf.Type().Elem())
							rf.Set(value)
							gdi.set(rf.Type(), value.Interface()) //must understand the reflect type and reflect value and interface{} relation
							gdi.build(value)
							gdi.log(fmt.Sprintf("autocreate %v inject by type the field %v for %v ", ftype, fieldName, v.Type()))
						}
					}
				}
			}

			setPrivateField(v, i)

			//gdi.panic(fmt.Sprintf("the field '%v' of '%v' inject faild,can't inject field not export", v.Elem().Type().Field(i).Name, v.Elem().Type().String()))
		}

	}
}

func GetPtrUnExportFiled(s interface{}, filed string) reflect.Value {
	v := reflect.ValueOf(s).Elem().FieldByName(filed)
	// 必须要调用 Elem()
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

func GetPtrUnExportFiledByIndex(s interface{}, index int) reflect.Value {
	v := reflect.ValueOf(s).Elem().Field(index)
	// 必须要调用 Elem()
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

func SetPtrUnExportFiledByIndex(s interface{}, index int, val interface{}) error {
	v := GetPtrUnExportFiledByIndex(s, index)
	rv := reflect.ValueOf(val)
	if v.Kind() != v.Kind() {
		return fmt.Errorf("invalid kind, expected kind: %v, got kind:%v", v.Kind(), rv.Kind())
	}

	v.Set(rv)
	return nil
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
			} else if len(vals) == 2 && vals[1].Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				if vals[1].Interface() != nil {
					gdi.panic(fmt.Sprintf("(ERROR) create %v '%v'", outType, vals[1].Interface().(string)))
				}
				gdi.typeToValues[outType] = vals[0]

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

type HandlerFactory struct {
	process map[string]*methodInfo
	lock    *sync.Mutex
}

type methodInfo struct {
	mt reflect.Method
	mv reflect.Value
}

func NewHandlerFactory() *HandlerFactory {
	return &HandlerFactory{
		process: make(map[string]*methodInfo, 100),
		lock:    &sync.Mutex{},
	}
}

func (p *HandlerFactory) Call(name string, args ...interface{}) (interface{}, error) {
	if m, ok := p.process[name]; ok {
		method := m.mt
		t1 := method.Type

		if len(args) != t1.NumIn()-1 {
			return nil, fmt.Errorf("The number of parameters is different!")
		}
		var vals []reflect.Value
		for i := 0; i < t1.NumIn(); i++ {
			for j := 0; j < len(args); j++ {
				if reflect.TypeOf(args[j]) == t1.In(i) {
					vals = append(vals, reflect.ValueOf(args[j]))
				}
			}
		}
		values := m.mv.Call(vals)
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
		} else {
			if len(values) == 1 {
				return values[0].Interface(), nil
			}
		}
		return values[len(values)-1].Interface(), nil
	} else {
		return nil, fmt.Errorf("%s not found", name)
	}

}

type Error struct {
	msg string
}

func (g *Error) Error() string {
	return g.msg
}

func NewError(msg string) *Error {
	return &Error{
		msg: msg,
	}
}

func (p *HandlerFactory) AddHandler(v interface{}) {
	t1 := reflect.TypeOf(v)
	v1 := reflect.ValueOf(v)
	if v1.IsNil() {
		panic("传递的对象不能为空！")
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if t1.Kind() == reflect.Ptr && t1.Elem().Kind() == reflect.Struct {
		for i := 0; i < t1.NumMethod(); i++ {
			pt := t1.Method(i).Type
			ok := false
			//if pt.NumOut() > 1 {
			//	panic("函数的返回值只能是一个，并且是error类型，如需返回值需要参数中传递对象指针作为返回用途")
			//}
			if pt.NumOut() > 0 {
				if !pt.Out(pt.NumOut() - 1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
					panic("函数的返回值只能是error类型，如需返回值需要参数中传递对象指针作为返回用途")
				}
			}
			for j := 1; j < pt.NumIn(); j++ {
				if pt.In(j).Kind() == reflect.Ptr && pt.In(j).Elem().Kind() == reflect.Struct {
					ok = true
				}
				if pt.NumIn() > 2 {
					for k := j + 1; k < pt.NumIn(); k++ {
						if pt.In(j).Kind() == pt.In(k).Kind() {
							panic("函数中不能带有相同类型的参数，如有同类型的参数需要进行类型封装，再进行传递")
						}
					}
				}

			}
			if !ok {
				//panic("你的函数参数中必须带有一个结构体的指针用于返回值！")
			}
			info := &methodInfo{
				mt: t1.Method(i),
				mv: v1.Method(i),
			}
			p.process[t1.Method(i).Name] = info
		}
	} else {
		panic("注册的Handler对象必须是struct指针")
	}

}
