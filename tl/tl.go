package tl

import (
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
