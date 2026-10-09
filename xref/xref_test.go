package xref

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Apple struct{ Value int }

func TestNewObject(t *testing.T) {
	t.Run("normal value", func(t *testing.T) {
		var valueFunc = func(Apple) {}
		obj := NewObject[Apple](reflect.TypeOf(valueFunc).In(0))
		assert.IsType(t, Apple{}, obj)
	})

	t.Run("normal pointer", func(t *testing.T) {
		var ptrFunc = func(*Apple) {}
		obj := NewObject[*Apple](reflect.TypeOf(ptrFunc).In(0))
		assert.IsType(t, &Apple{}, obj)
		assert.NotNil(t, obj)
		assert.Zero(t, obj.Value)
	})
}
