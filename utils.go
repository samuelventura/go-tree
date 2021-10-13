package tree

import "runtime"

func stacktrace() string {
	array := make([]byte, 2048)
	s := runtime.Stack(array, false)
	return string(array[0:s])
}
