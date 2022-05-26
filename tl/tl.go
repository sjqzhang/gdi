package tl

import (
	"reflect"
	"unsafe"
)

func Typelinks() (sections []unsafe.Pointer, offset [][]int32) {
	return typelinks()
}

//go:linkname typelinks reflect.typelinks
func typelinks() (sections []unsafe.Pointer, offset [][]int32)

func Add(p unsafe.Pointer, x uintptr, whySafe string) unsafe.Pointer {
	return add(p, x, whySafe)
}

func add(p unsafe.Pointer, x uintptr, whySafe string) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

func GetAllTypes() []reflect.Type {
	sections, offsets := Typelinks()
	var tys []reflect.Type
	for i, base := range sections {
		for _, offset := range offsets[i] {
			typeAddr := Add(base, uintptr(offset), "")
			typ := reflect.TypeOf(*(*interface{})(unsafe.Pointer(&typeAddr)))
			tys = append(tys, typ)
		}
	}
	return tys
}
