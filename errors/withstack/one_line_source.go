package withstack

import (
	"fmt"
	"path/filepath"
	"strings"

	pkgErr "github.com/pkg/errors"

	"github.com/StevenACoffman/anotherr/errors/errbase"
)

// GetOneLineSource extracts the file/line/function information
// of the topmost caller in the innermost recorded stack trace.
// The filename is simplified to remove the path prefix.
// This is used e.g. to populate the "source" field in
// PostgreSQL errors.
func GetOneLineSource(err error) (file string, line int, fn string, ok bool) {
	// We want the innermost entry: start by recursing.
	if c := errbase.UnwrapOnce(err); c != nil {
		if file, line, fn, ok = GetOneLineSource(c); ok {
			return
		}
	}
	// If we reach this point, we haven't found anything in the cause so
	// far. Look at the current level.

	// If we have a stack trace in the style of github.com/pkg/errors
	// (either from there or our own withStack), use it.
	if st, ok := err.(errbase.StackTraceProvider); ok {
		return getOneLineSourceFromPkgStack(st.StackTrace())
	}

	// If we have flattened a github.com/pkg/errors-style stack
	// trace to a string, it will happen in the error's safe details
	// and we need to parse it.

	details := getDetails(err)
	if len(details) > 0 {
		return getOneLineSourceFromPrintedStack(details[0])
	}

	// No conversion available - no stack trace.
	return "", 0, "", false
}

func getDetails(err error) []string {
	// For convenience, we also know how to extract stack traces
	// in the style of github.com/pkg/errors.
	if st, ok := err.(interface{ StackTrace() pkgErr.StackTrace }); ok {
		return []string{fmt.Sprintf("%+v", st.StackTrace())}
	}

	return nil
}

func getOneLineSourceFromPkgStack(
	st errbase.StackTrace,
) (file string, line int, fn string, ok bool) {
	if len(st) > 0 {
		st = st[:1]
		// Note: the stack trace logic changed between go 1.11 and 1.12.
		// Trying to analyze the frame PCs point-wise will cause
		// the output to change between the go versions.
		stS := fmt.Sprintf("%+v", st)

		return getOneLineSourceFromPrintedStack(stS)
	}

	return "", 0, "", false
}

func getOneLineSourceFromPrintedStack(st string) (file string, line int, fn string, ok bool) {
	// We only need 3 lines: the function/file/line info will be on the
	// first two lines. See parsePrintedStack() for details.
	lines := strings.SplitN(strings.TrimSpace(st), "\n", 3)
	if len(lines) > 0 {
		_, file, line, fnName := parsePrintedStackEntry(lines, 0)
		if fnName != "unknown" {
			_, fn = functionName(fnName)
		}

		return filepath.Base(file), line, fn, true
	}

	return "", 0, "", false
}
