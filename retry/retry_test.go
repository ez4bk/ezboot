package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExponential(t *testing.T) {
	t.Parallel()

	t.Run("error once", func(t *testing.T) {
		err := Exponential(1, func() error {
			return errors.New("error case")
		})
		assert.Error(t, err)
	})

	t.Run("no error", func(t *testing.T) {
		err := Exponential(3, func() error {
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("error 3th", func(t *testing.T) {
		count := 0
		start := time.Now()
		err := Exponential(3, func() error {
			count++
			t.Logf("%d: %s", count, time.Now().Sub(start))
			return errors.New("error case")
		})
		assert.Error(t, err)
		assert.Equal(t, 3, count)
		assert.GreaterOrEqual(t, time.Now().Sub(start), 2*time.Second)
	})

	t.Run("error 5th", func(t *testing.T) {
		count := 0
		start := time.Now()
		err := Exponential(5, func() error {
			count++
			t.Logf("%d: %s", count, time.Now().Sub(start))
			return errors.New("error case")
		})
		assert.Error(t, err)
		assert.Equal(t, 5, count)
		assert.GreaterOrEqual(t, time.Now().Sub(start), 3*time.Second)
	})
}

func TestConstant(t *testing.T) {
	t.Parallel()

	t.Run("error once", func(t *testing.T) {
		err := Constant(1, func() error {
			return errors.New("error case")
		}, 500*time.Millisecond)
		assert.Error(t, err)
	})

	t.Run("no error", func(t *testing.T) {
		err := Constant(3, func() error {
			return nil
		}, 500*time.Millisecond)
		assert.NoError(t, err)
	})

	t.Run("error 3th", func(t *testing.T) {
		count := 0
		start := time.Now()
		err := Constant(3, func() error {
			count++
			t.Logf("%d: %s", count, time.Now().Sub(start))
			return errors.New("error case")
		}, 500*time.Millisecond)
		assert.Error(t, err)
		assert.Equal(t, 3, count)
		assert.GreaterOrEqual(t, time.Now().Sub(start), 1*time.Second+500*time.Millisecond)
	})

	t.Run("error 5th", func(t *testing.T) {
		count := 0
		start := time.Now()
		err := Constant(5, func() error {
			count++
			t.Logf("%d: %s", count, time.Now().Sub(start))
			return errors.New("error case")
		}, 500*time.Millisecond)
		assert.Error(t, err)
		assert.Equal(t, 5, count)
		assert.GreaterOrEqual(t, time.Now().Sub(start), 2*time.Second+500*time.Millisecond)
	})
}
