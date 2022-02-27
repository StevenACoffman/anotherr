package errors

import (
	"fmt"
	"runtime"

	"github.com/StevenACoffman/anotherr/errors/errbase"
)

// stack represents a stack of program counters. This mirrors the
// (non-exported) type of the same name in github.com/pkg/errors.
type stack []uintptr

// Format mirrors the code in github.com/pkg/errors.
func (s *stack) Format(st fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case st.Flag('+'):
			for _, pc := range *s {
				f := errbase.StackFrame(pc)
				fmt.Fprintf(st, "\n%+v", f)
			}
		}
	}
}

// StackTrace mirrors the code in github.com/pkg/errors.
func (s *stack) StackTrace() errbase.StackTrace {
	f := make([]errbase.StackFrame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = errbase.StackFrame((*s)[i])
	}

	return f
}

// callers mirrors the code in github.com/pkg/errors,
// but makes the depth customizable.
func callers(depth int) *stack {
	const numFrames = 32
	var pcs [numFrames]uintptr
	n := runtime.Callers(2+depth, pcs[:])
	var st stack = pcs[0:n]

	return &st
}
