//build +testing

package ini

import (
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var (
	debugLogFn func(v ...interface{})
	errorLogFn func(v ...interface{})
)

func pushlog(t *testing.T) func() {
	d, e := debugLogFn, errorLogFn
	debugLogFn = t.Log
	errorLogFn = t.Error
	return func() {
		debugLogFn, errorLogFn = d, e
	}
}

// Info

func debugPrefix(depth, minlen int) string {
	pc, file, line, _ := runtime.Caller(depth + 2)
	fname := "<unknown>"
	if fn := runtime.FuncForPC(pc); fn != nil {
		fname = path.Base(fn.Name())
	}

	prefix := fmt.Sprint(filepath.Base(file), ":", line, ":", fname, ": ")
	if len(prefix) < minlen {
		prefix = prefix + strings.Repeat(" ", minlen-len(prefix))
	}
	return prefix
}

func dlog(depth int, v ...interface{}) {
	if debugLogFn != nil {
		debugLogFn("D", debugPrefix(depth, 64)+fmt.Sprint(v...))
	}
}

func dlogf(depth int, format string, v ...interface{}) {
	if debugLogFn != nil {
		debugLogFn("D", debugPrefix(depth, 64)+fmt.Sprintf(format, v...))
	}
}

// Errors

func elog(depth int, v ...interface{}) {
	if errorLogFn != nil {
		errorLogFn("E", debugPrefix(depth, 52)+fmt.Sprint(v...))
	}
}

func elogf(depth int, format string, v ...interface{}) {
	if errorLogFn != nil {
		errorLogFn("E", debugPrefix(depth, 52)+fmt.Sprintf(format, v...))
	}
}
