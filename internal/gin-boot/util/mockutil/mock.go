// Copyright (C) 2019 LambdaCal Inc.

package mockutil

import (
	"bou.ke/monkey"
)

// Cancel Cancel
type Cancel = func()

// Mock Mock
// only use this in test or ide envrionment
func Mock(target interface{}, newT interface{}) Cancel {
	monkey.Patch(target, newT)
	return func() {
		monkey.Unpatch(target)
	}
}

// ResetAll ResetAll
func ResetAll() {
	monkey.UnpatchAll()
}
