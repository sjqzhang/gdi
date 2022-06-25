package gdi

import (
	"errors"
	"fmt"
	"github.com/sjqzhang/gdi/tl"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

var globalGDI *GDIPool

type logLevel int

const (
	logLevelInfo logLevel = iota
	logLevelWarning
	logLevelError
	logLevelPanic
	logLevelExit
)

//GDIPool 依赖注入容器
type GDIPool struct {
	debug                 bool
	scanPkgPaths          []string
	ignoreInterface       bool
	ignorePrivate         bool
	creator               map[reflect.Type]interface{}
	creatorLocker         sync.RWMutex
	typeToValuesReadOnly  map[reflect.Type]reflect.Value
	typeToValues          map[reflect.Type]reflect.Value
	allTypesToValues      map[reflect.Type]reflect.Value
	namesToValues         map[string]reflect.Value
	namesToValuesReadOnly map[string]reflect.Value
	interfaceToImplements map[string]string

	ttvLocker  sync.RWMutex
	autoCreate bool
}

var consoleLog = log.New(os.Stdout, "[gdi] ", log.LstdFlags)

func init() {
	globalGDI = NewGDIPool()
}

//NewGDIPool 创建依赖容器
func NewGDIPool() *GDIPool {
	modName := "main"
	pool := &GDIPool{
		debug:                 true,
		scanPkgPaths:          []string{modName},
		ignoreInterface:       false,
		autoCreate:            true,
		ignorePrivate:         false,
		creator:               make(map[reflect.Type]interface{}),
		creatorLocker:         sync.RWMutex{},
		allTypesToValues:      make(map[reflect.Type]reflect.Value),
		typeToValues:          make(map[reflect.Type]reflect.Value),
		typeToValuesReadOnly:  make(map[reflect.Type]reflect.Value),
		namesToValues:         make(map[string]reflect.Value),
		namesToValuesReadOnly: make(map[string]reflect.Value),
		interfaceToImplements: make(map[string]string),
		ttvLocker:             sync.RWMutex{},
	}
	for _, t := range GetAllTypes() {
		pool.allTypesToValues[t] = reflect.ValueOf(nil)
	}
	return pool
}

//Register 用于注册自己的业务代码
func Register(funcObjOrPtrs ...interface{}) {
	globalGDI.Register(funcObjOrPtrs...)

}

// RegisterReadOnly 用于注册第三方代码，非自己的业务代码
func RegisterReadOnly(funcObjOrPtrs ...interface{}) {
	globalGDI.RegisterReadOnly(funcObjOrPtrs...)

}

//func IgnoreInterfaceInject(isIgnoreInterfaceInject bool) {
//	globalGDI.ignoreInterface = isIgnoreInterfaceInject
//}

//func ScanPkgPaths(scanPaths ...string) {
//	globalGDI.scanPkgPaths = scanPaths
//}

// AutoCreate 是否自动创建对象，true:自动创建，false:非自动创建 default:true
func AutoCreate(autoCreate bool) {
	if autoCreate {
		globalGDI.Debug(autoCreate)
	}
	globalGDI.AutoCreate(autoCreate)
}

// IgnorePrivate 是否对非公开属性进行注入？
func IgnorePrivate(isIgnorePrivate bool) {
	globalGDI.ignorePrivate = isIgnorePrivate
}

// Get 通过类型或名称从容器中获取对象
func Get(t interface{}) (value interface{}) {
	return globalGDI.Get(t)
}

// Invoke 函数参数自动注入
func Invoke(t interface{}) (interface{}, error) {
	return globalGDI.Invoke(t)
}

// DI 自动依懒注入
func DI(pointer interface{}) error {
	return globalGDI.DI(pointer)
}

// MapToImplement 设定接品与实现的映射关系 Example：gdi.MapToImplement(&AA{},&Q{}
func MapToImplement(pkgToFieldInteface interface{}, pkgImplement interface{}) error {
	return globalGDI.MapToImplement(pkgToFieldInteface, pkgImplement)
}

//GetAllTypes 获取所有类型
func GetAllTypes() []reflect.Type {
	return globalGDI.GetAllTypes()
}

//GetWithCheck 从容器中获取值
func GetWithCheck(t interface{}) (value interface{}, ok bool) {
	return globalGDI.GetWithCheck(value)
}

//Init 在使用前必须先调用它
func Init() {
	globalGDI.Init()
}

//func (gdi *GDIPool) IgnoreInterfaceInject(isIgnoreInterfaceInject bool) {
//	gdi.ignoreInterface = isIgnoreInterfaceInject
//}
//ScanPkgPaths deprecated
func (gdi *GDIPool) ScanPkgPaths(scanPaths ...string) {
	gdi.scanPkgPaths = append(gdi.scanPkgPaths, scanPaths...)
}

func (gdi *GDIPool) IgnorePrivate(isIgnorePrivate bool) {
	gdi.ignorePrivate = isIgnorePrivate
}

//Register 用于注册自己的业务代码
func (gdi *GDIPool) Register(funcObjOrPtrs ...interface{}) {
	for i := range funcObjOrPtrs {
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

// RegisterReadOnly 用于注册第三方代码，非自己的业务代码
func (gdi *GDIPool) RegisterReadOnly(funcObjOrPtrs ...interface{}) {
	for i := range funcObjOrPtrs {
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
			gdi.setReadOnly(ftype, funcObjOrPtr)
			continue
		}
		outType, err := gdi.parsePoolFunc(funcObjOrPtr)
		if err != nil {
			gdi.panic(err.Error())
		}
		if ftype.Kind() == reflect.Func {
			if ftype.NumIn() == 0 {
				gdi.setReadOnly(outType, funcObjOrPtr)
			} else {
				gdi.creator[outType] = funcObjOrPtr
			}
		}
	}
}

//Init 在使用前必须先调用它
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
		gdi.build(v, true)
	}
	for _, v := range gdi.namesToValues {
		gdi.build(v, true)
	}
	//if err:=gdi.checkPoolNil();err!=nil {
	//	panic(err)
	//}
	return gdi
}
func (gdi *GDIPool) checkPoolNil() error {
	for t, v := range gdi.all() {
		if v.Elem().Kind() != reflect.Struct {
			continue
		}
		for i := 0; i < v.Elem().NumField(); i++ {
			field := v.Elem().Field(i)
			fieldName := v.Type().Elem().Field(i).Name
			if (field.Kind() == reflect.Interface || field.Kind() == reflect.Ptr) && field.IsNil() {
				return fmt.Errorf("fieldname:%v of %v is null", fieldName, t.String())
			}
		}
	}
	gdi.ttvLocker.Lock()
	defer gdi.ttvLocker.Unlock()
	for name, v := range gdi.namesToValues {
		if v.Elem().Kind() != reflect.Struct {
			continue
		}
		for i := 0; i < v.Elem().NumField(); i++ {
			field := v.Elem().Field(i)
			fieldName := v.Type().Elem().Field(i).Name
			if (field.Kind() == reflect.Interface || field.Kind() == reflect.Ptr) && field.IsNil() {
				return fmt.Errorf("fieldname:%v of %v is null", fieldName, name)
			}
		}
	}
	return nil
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
func (gdi *GDIPool) pkgInScanPaths(pkg string) bool {
	for _, p := range gdi.scanPkgPaths {
		if p == pkg {
			return true
		}
	}
	return false
}

func (gdi *GDIPool) injectLog(fieldName string, field reflect.Value, vStruct reflect.Value, pkgPath string, level logLevel) {
	switch level {
	case logLevelWarning:
		gdi.warn(fmt.Sprintf("inject fieldName:%v->%v of %v pkgPath:%v", fieldName, field.Type(), vStruct.Type(), pkgPath))
	case logLevelError:
		gdi.error(fmt.Sprintf("inject fieldName:%v->%v of %v pkgPath:%v", fieldName, field.Type(), vStruct.Type(), pkgPath))
	case logLevelPanic:
		gdi.panic(fmt.Sprintf("inject fieldName:%v->%v of %v pkgPath:%v", fieldName, field.Type(), vStruct.Type(), pkgPath))
	case logLevelExit:
		gdi.error(fmt.Sprintf("inject fieldName:%v->%v of %v pkgPath:%v", fieldName, field.Type(), vStruct.Type(), pkgPath))
		os.Exit(1)
	default:
		gdi.log(fmt.Sprintf("inject fieldName:%v->%v of %v pkgPath:%v", fieldName, field.Type(), vStruct.Type(), pkgPath))
	}
}

func (gdi *GDIPool) build(v reflect.Value, exitOnError bool) {
	if v.Elem().Kind() != reflect.Struct {
		return
	}
	n := &node{}
	n.name = v.Type().String()
	g.add(n)
	for i := 0; i < v.Elem().NumField(); i++ {
		nf := &nodeItem{}
		n.addFiled(nf)
		field := v.Elem().Field(i)
		pkgPath := v.Type().Elem().PkgPath()
		fieldName := v.Type().Elem().Field(i).Name
		nf.fieldName = fmt.Sprintf(`f%v#%v`, i, fieldName)
		nf.fieldType = v.Type().Elem().Field(i).Type.String()
		_ = pkgPath
		if field.Kind() != reflect.Interface && field.Kind() != reflect.Ptr {
			continue
		}
		if !field.CanSet() {
			if gdi.ignorePrivate {
				continue
			}
			field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
		}
		name, ok := gdi.getTagAttr(v.Type().Elem().Field(i), "name")
		if ok && name != "" { // struct tag inject:name:hello
			if value, ok := gdi.getByName(name); ok {
				//n.addEdge(&edge{from: nf.fieldType, to: value.Type().String()})
				n.addEdge(&edge{from: fmt.Sprintf(`"%v":f%v`, v.Type().String(), i), to: value.Type().String()})
				field.Set(value)
				gdi.injectLog(fieldName, field, v, pkgPath, logLevelInfo)
			} else {
				gdi.panic(fmt.Sprintf("name:%v type:%v object not found", name, field.Type()))
			}
		}
		if field.Kind() == reflect.Interface { // interface
			if !field.IsNil() {
				continue
			}
			if im, err := gdi.getByInterface(field.Type(), fieldName, v); err == nil {
				field.Set(im)
				//n.addEdge(&edge{from: fmt.Sprintf("%v:f%v", nf.fieldType,i), to: im.Type().String()})
				n.addEdge(&edge{from: fmt.Sprintf(`"%v":f%v`, v.Type().String(), i), to: im.Type().String()})
				gdi.injectLog(fieldName, field, v, pkgPath, logLevelInfo)
				continue
			} else {
				if field.Type().String() != "interface {}" && field.Type().String() != "error" {
					gdi.error(err.Error())
					if exitOnError {
						gdi.injectLog(fieldName, field, v, pkgPath, logLevelExit)
					} else {
						gdi.injectLog(fieldName, field, v, pkgPath, logLevelError)
					}

				} else {
					gdi.warn(fmt.Sprintf("\u001B[1;31mignore type:%v fieldName:%v of %v pkgPath:%v\u001B[0m", field.Type(), fieldName, v.Type(), pkgPath))
					continue
				}
			}
			if exitOnError {
				gdi.injectLog(fieldName, field, v, pkgPath, logLevelExit)
			} else {
				gdi.injectLog(fieldName, field, v, pkgPath, logLevelError)
			}
			continue
		}
		if im, ok := gdi.get(field.Type()); ok { // by type
			field.Set(im)
			//n.addEdge(&edge{from: nf.fieldType, to: im.Type().String()})
			n.addEdge(&edge{from: fmt.Sprintf(`"%v":f%v`, v.Type().String(), i), to: im.Type().String()})
			//n.addFiled(nf)
			gdi.injectLog(fieldName, field, v, pkgPath, logLevelInfo)
			continue
		} else {
			if gdi.autoCreate {
				value := reflect.New(field.Type().Elem())
				field.Set(value)
				//n.addEdge(&edge{from: nf.fieldType, to: value.Type().String()})
				n.addEdge(&edge{from: fmt.Sprintf(`"%v":f%v`, v.Type().String(), i), to: value.Type().String()})
				gdi.injectLog(fieldName, field, v, pkgPath, logLevelInfo)
				gdi.warn(fmt.Sprintf("\u001B[1;35mautoCreate\u001B[0m type:%v fieldName:%v of %v", field.Type(), fieldName, v.Type()))
				gdi.set(field.Type(), value.Interface())
				gdi.build(value, exitOnError)
			}
		}
		if field.IsNil() {
			if exitOnError {
				gdi.injectLog(fieldName, field, v, pkgPath, logLevelExit)
			} else {
				gdi.injectLog(fieldName, field, v, pkgPath, logLevelError)
			}
		}
		//n.addFiled(nf)
	}
}

//SetStructPtrUnExportedStrField 通过结构体指针设置字段属性
func SetStructPtrUnExportedStrField(source interface{}, fieldName string, fieldVal interface{}) (err error) {
	v := GetStructPtrUnExportedField(source, fieldName)
	rv := reflect.ValueOf(fieldVal)
	if v.Kind() != rv.Kind() {
		return fmt.Errorf("invalid kind: expected kind %v, got kind: %v", v.Kind(), rv.Kind())
	}

	v.Set(rv)
	return nil
}

//SetStructUnExportedStrField 设置结构体中未导出属性
func SetStructUnExportedStrField(source interface{}, fieldName string, fieldVal interface{}) (addressableSourceCopy reflect.Value, err error) {
	var accessableField reflect.Value
	accessableField, addressableSourceCopy = GetStructUnExportedField(source, fieldName)
	rv := reflect.ValueOf(fieldVal)
	if accessableField.Kind() != rv.Kind() {
		return addressableSourceCopy, fmt.Errorf("invalid kind: expected kind %v, got kind: %v", addressableSourceCopy.Kind(), rv.Kind())
	}
	accessableField.Set(rv)
	return
}

//GetStructPtrUnExportedField 通过结构体指针获取未导出属性
func GetStructPtrUnExportedField(source interface{}, fieldName string) reflect.Value {
	v := reflect.ValueOf(source).Elem().FieldByName(fieldName)
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

//GetStructUnExportedField 获取结构构未导出属性
func GetStructUnExportedField(source interface{}, fieldName string) (accessableField, addressableSourceCopy reflect.Value) {
	v := reflect.ValueOf(source)
	// since source is not a ptr, get an addressable copy of source to modify it later
	addressableSourceCopy = reflect.New(v.Type()).Elem()
	addressableSourceCopy.Set(v)
	accessableField = addressableSourceCopy.FieldByName(fieldName)
	accessableField = reflect.NewAt(accessableField.Type(), unsafe.Pointer(accessableField.UnsafeAddr())).Elem()
	return
}

//Debug 是否开启调试信息
func (gdi *GDIPool) Debug(isDebug bool) {
	gdi.debug = isDebug
}

func Graph() string {
	return globalGDI.Graph()
}

//Debug 是否开启调试信息
func Debug(isDebug bool) {
	globalGDI.debug = isDebug
}

// AutoCreate 是否自动创建对象，true:自动创建，false:非自动创建 default:true
func (gdi *GDIPool) AutoCreate(create bool) {
	gdi.autoCreate = create
}

//GetAllTypes 获取所有类型
func (gdi *GDIPool) GetAllTypes() []reflect.Type {
	sections, offsets := tl.Typelinks()
	var tys []reflect.Type
	for i, base := range sections {
		for _, offset := range offsets[i] {
			typeAddr := tl.Add(base, uintptr(offset), "")
			typ := reflect.TypeOf(*(*interface{})(unsafe.Pointer(&typeAddr)))
			tys = append(tys, typ)
		}
	}
	return tys
}

// DI 自动依懒注入
func (gdi *GDIPool) DI(pointer interface{}) (e error) {
	defer func() {
		if err := recover(); err != nil {
			gdi.warn(fmt.Sprintf("%v", err))
			e = fmt.Errorf(fmt.Sprintf("%v", err))
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
	gdi.build(result, false)
	return e
}

// Invoke 函数参数自动注入
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
			return nil, errors.New("(WARNING) can't inject all parameters")
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

// Get 通过类型或名称从容器中获取对象
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

//GetWithCheck 从容器中获取值
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
	if result, ok = gdi.typeToValues[t]; !ok {
		result, ok = gdi.typeToValuesReadOnly[t]
	}
	return
}

// MapToImplement 设定接品与实现的映射关系 Example：gdi.MapToImplement(&AA{},&Q{}
func (gdi *GDIPool) MapToImplement(pkgToFieldInteface interface{}, pkgImplement interface{}) error {
	if reflect.TypeOf(pkgToFieldInteface).Kind() != reflect.Ptr || reflect.TypeOf(pkgImplement).Kind() != reflect.Ptr {
		return fmt.Errorf("pkgToFieldInteface and pkgImplement must be a Ptr")
	}
	gdi.interfaceToImplements[reflect.TypeOf(pkgToFieldInteface).String()] = reflect.TypeOf(pkgImplement).String()
	return nil
}

func (gdi *GDIPool) getByInterface(i reflect.Type, fieldName string, v reflect.Value) (value reflect.Value, err error) {
tag:
	cnt := 0
	var values []reflect.Value
	for t, v2 := range gdi.all() {
		if t.Implements(i) {
			cnt++
			value = v2
			values = append(values, v2)
			for tface, timpl := range gdi.interfaceToImplements {
				if tface == v.Type().String() && t.String() == timpl {
					return value, nil
				}
			}
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
		msg := fmt.Sprintf("there is one more object impliment %v interface [%v].please use gdi.MapToImplement to set Interface->Implements.", i.Name(), strings.Join(msgs, ","))
		return reflect.Value{}, fmt.Errorf(msg)
	}
	bflag := false
	for t := range gdi.allTypesToValues {
		if t.Kind() != reflect.Ptr {
			continue
		}
		if t.Elem().Kind() != reflect.Struct {
			continue
		}
		//if strings.ToLower(strings.TrimSpace(t.Elem().PkgPath()))=="main" {
		//  fmt.Sprintf("enter")
		//}
		if t.Implements(i) {
			if gdi.autoCreate {
				value = reflect.New(t.Elem())
				gdi.warn(fmt.Sprintf("\u001B[1;35mautoCreate\u001B[0m  type:%v fieldName:%v of %v", t, fieldName, v.Type()))
				gdi.set(t, value.Interface())
				bflag = true
				//return value, nil
			}
		}
	}
	if bflag {
		goto tag
	}

	return reflect.Value{}, fmt.Errorf("interface type:%v fieldName:%v of %v not found.please use gdi.MapToImplement to set Interface->Implements", i.Name(), fieldName, v)
}

func (gdi *GDIPool) getByName(name string) (result reflect.Value, ok bool) {
	gdi.ttvLocker.RLock()
	defer gdi.ttvLocker.RUnlock()
	if result, ok = gdi.namesToValues[name]; !ok {
		result, ok = gdi.namesToValuesReadOnly[name]
	}
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
func (gdi *GDIPool) error(msg string) {
	colorRed := "\033[1;31m"
	colorNormal := "\033[0m"
	consoleLog.Println(fmt.Sprintf("%v%v%v", colorRed, "ERROR: "+msg, colorNormal))
}
func (gdi *GDIPool) panic(msg string) {
	gdi.error(msg)
	panic("")
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

func (gdi *GDIPool) setReadOnly(outType reflect.Type, f interface{}) {
	if f != nil {
		if reflect.TypeOf(f).Kind() == reflect.Func {
			vals := gdi.create(f)
			if len(vals) == 1 {
				gdi.typeToValuesReadOnly[outType] = vals[0]
			} else if len(vals) == 2 && vals[1].Kind() == reflect.String {
				if _, ok := gdi.namesToValuesReadOnly[vals[1].Interface().(string)]; ok {
					gdi.panic(fmt.Sprintf("double register name: '%v'", vals[1].Interface().(string)))
				}
				name := vals[1].Interface().(string)
				gdi.namesToValuesReadOnly[name] = vals[0]
				gdi.log(fmt.Sprintf("register by name, name:%v type:%v success", name, vals[0].Type()))
				return
			} else if len(vals) == 2 && vals[1].Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				if vals[1].Interface() != nil {
					gdi.panic(fmt.Sprintf("(ERROR) create %v '%v'", outType, vals[1].Interface().(string)))
				}
				gdi.typeToValuesReadOnly[outType] = vals[0]
			}
			gdi.log(fmt.Sprintf("register by type, type:%v pkgPath:%v success", outType, outType.Elem().PkgPath()))
		} else if reflect.TypeOf(f).Kind() == reflect.Ptr {
			gdi.typeToValuesReadOnly[outType] = reflect.ValueOf(f)
			gdi.log(fmt.Sprintf("register by type, type:%v pkgPath:%v success", outType, outType.Elem().PkgPath()))
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
			gdi.log(fmt.Sprintf("register by type, type:%v pkgPath:%v success", outType, outType.Elem().PkgPath()))
		} else if reflect.TypeOf(f).Kind() == reflect.Ptr {
			gdi.typeToValues[outType] = reflect.ValueOf(f)
			gdi.log(fmt.Sprintf("register by type, type:%v PkgPath:%v success", outType, outType.Elem().PkgPath()))
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
		e = fmt.Errorf("%v it's not a func", f)
		return
	}
	if ftype.NumOut() == 0 {
		e = fmt.Errorf("%v return values should be a pointer", f)
		return
	}
	if ftype.NumOut() > 2 {
		e = errors.New("return values should be less 2")
		return
	}
	outType = ftype.Out(0)
	if outType.Kind() != reflect.Ptr && outType.Kind() != reflect.Interface {
		e = errors.New("the first return value must be an object pointer")
		return
	}
	return
}
