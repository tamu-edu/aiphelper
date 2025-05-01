package utils

import (
	"fmt"
)

// DebugEnabled controls whether debug messages are printed
var DebugEnabled bool

// Debug prints a message if debug mode is enabled
func Debug(format string, a ...interface{}) {
	if DebugEnabled {
		fmt.Printf("[DEBUG] "+format+"\n", a...)
	}
}
