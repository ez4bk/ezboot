package xref

import "reflect"

// NewObject 根据范型创建一个对象出来
func NewObject[T any](rt reflect.Type) T {
	var isPtr bool
	for rt.Kind() == reflect.Ptr {
		isPtr = true
		rt = rt.Elem()
	}

	rv := reflect.New(rt)
	if !isPtr {
		rv = rv.Elem()
	}
	return rv.Interface().(T)
}
