package jrpc

import "reflect"

type MethodRegistry interface {
	GetByName(name string) (reflect.Value, bool)
}
