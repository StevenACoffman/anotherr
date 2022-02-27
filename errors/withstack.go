package errors

import "github.com/StevenACoffman/anotherr/errors/withstack"

// This file mirrors the WithStack functionality from
// github.com/pkg/errors. We would prefer to reuse the withStack
// struct from that package directly (the library recognizes it well)
// unfortunately github.com/pkg/errors does not enable client code to
// customize the depth at which the stack trace is captured.

// WithStack annotates err with a stack trace at the point WithStack was
// called.
//
// Detail is shown:
// - via `errors.GetSafeDetails()`
// - when formatting with `%+v`.
// - in Sentry reports.
// - when innermost stack capture, with `errors.GetOneLineSource()`.
func WithStack(err error) error { return withstack.WithStackDepth(err, 1) }

// WithStackDepth annotates err with a stack trace starting from the
// given call depth. The value zero identifies the caller
// of WithStackDepth itself.
// See the documentation of WithStack() for more details.
func WithStackDepth(err error, depth int) error { return withstack.WithStackDepth(err, depth+1) }

// ReportableStackTrace aliases the type of the same name in the sentry
// package. This is used by SendReport().
type ReportableStackTrace = withstack.ReportableStackTrace

// GetOneLineSource extracts the file/line/function information
// of the topmost caller in the innermost recorded stack trace.
// The filename is simplified to remove the path prefix.
//
// This is used e.g. to populate the "source" field in
// PostgreSQL errors in CockroachDB.
func GetOneLineSource(err error) (file string, line int, fn string, ok bool) {
	return withstack.GetOneLineSource(err)
}

// GetReportableStackTrace extracts a stack trace embedded in the
// given error in the format suitable for Sentry reporting.
//
// This supports:
// - errors generated by github.com/pkg/errors (either generated
//   locally or after transfer through the network),
// - errors generated with WithStack() in this package,
// - any other error that implements a StackTrace() method
//   returning a StackTrace from github.com/pkg/errors.
//
// Note: Sentry wants the oldest call frame first, so
// the entries are reversed in the result.
func GetReportableStackTrace(err error) *ReportableStackTrace {
	return withstack.GetReportableStackTrace(err)
}
