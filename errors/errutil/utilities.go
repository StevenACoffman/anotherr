// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package errutil

import (
	"fmt"

	"github.com/StevenACoffman/anotherr/errors/errbase"
	"github.com/StevenACoffman/anotherr/errors/withstack"
)

// New creates an error with a simple error message.
// A stack trace is retained.
//
// Note: the message string is assumed to not contain
// PII and is included in Sentry reports.
// Use errors.Newf("%s", <unsafestring>) for errors
// strings that may contain PII information.
//
// Detail output:
// - message via `Error()` and formatting using `%v`/`%s`/`%q`.
// - everything when formatting with `%+v`.
// - stack trace and message via `errors.GetSafeDetails()`.
// - stack trace and message in Sentry reports.
func New(msg string) error {
	return NewWithDepth(1, msg)
}

// NewWithDepth is like New() except the depth to capture the stack
// trace is configurable.
// See the doc of `New()` for more details.
func NewWithDepth(depth int, msg string) error {
	err := error(&leafError{msg})

	return withstack.WithStackDepth(err, 1+depth)
}

// Newf creates an error with a formatted error message.
// A stack trace is retained.
//
// Note: the format string is assumed to not contain
// PII and is included in Sentry reports.
// Use errors.Newf("%s", <unsafestring>) for errors
// strings that may contain PII information.
//
// See the doc of `New()` for more details.
func Newf(format string, args ...interface{}) error {
	return NewWithDepthf(1, format, args...)
}

// NewWithDepthf is like Newf() except the depth to capture the stack
// trace is configurable.
// See the doc of `New()` for more details.
func NewWithDepthf(depth int, format string, args ...interface{}) error {
	wrappedErr := fmt.Errorf(format, args...)

	return withstack.WithStackDepth(wrappedErr, 1+depth)
}

// Wrap wraps an error with a message prefix.
// A stack trace is retained.
//
// Note: the prefix string is assumed to not contain
// PII and is included in Sentry reports.
// Use errors.Wrapf(err, "%s", <unsafestring>) for errors
// strings that may contain PII information.
//
// Detail output:
// - original error message + prefix via `Error()` and formatting using
// `%v`/`%s`/`%q`.
// - everything when formatting with `%+v`.
// - stack trace and message via `errors.GetSafeDetails()`.
// - stack trace and message in Sentry reports.
func Wrap(err error, msg string) error {
	return WrapWithDepth(1, err, msg)
}

// WrapWithDepth is like Wrap except the depth to capture the stack
// trace is configurable.
// The the doc of `Wrap()` for more details.
func WrapWithDepth(depth int, err error, msg string) error {
	if err == nil {
		return nil
	}
	if msg != "" {
		err = WithMessage(err, msg)
	}

	return withstack.WithStackDepth(err, depth+1)
}

// Wrapf wraps an error with a formatted message prefix. A stack
// trace is also retained. If the format is empty, no prefix is added,
// but the extra arguments are still processed for reportable strings.
//
// Note: the format string is assumed to not contain
// PII and is included in Sentry reports.
// Use errors.Wrapf(err, "%s", <unsafestring>) for errors
// strings that may contain PII information.
//
// Detail output:
// - original error message + prefix via `Error()` and formatting using
// `%v`/`%s`/`%q`.
// - everything when formatting with `%+v`.
// - stack trace, format, and redacted details via `errors.GetSafeDetails()`.
// - stack trace, format, and redacted details in Sentry reports.
func Wrapf(err error, format string, args ...interface{}) error {
	return WrapWithDepthf(1, err, format, args...)
}

// WrapWithDepthf is like Wrapf except the depth to capture the stack
// trace is configurable.
// The the doc of `Wrapf()` for more details.
func WrapWithDepthf(depth int, err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	var errRefs []error
	for _, a := range args {
		if e, ok := a.(error); ok {
			errRefs = append(errRefs, e)
		}
	}
	if format != "" || len(args) > 0 {
		err = WithMessagef(err, format, args...)
	}

	return withstack.WithStackDepth(err, depth+1)
}

// withNewMessage is like withPrefix but the message completely
// overrides that of the underlying error.
type withNewMessage struct {
	cause   error
	message string
}

var (
	_ error         = (*withNewMessage)(nil)
	_ fmt.Formatter = (*withNewMessage)(nil)
)

func (l *withNewMessage) Error() string {
	return l.message
}

func (l *withNewMessage) Cause() error  { return l.cause }
func (l *withNewMessage) Unwrap() error { return l.cause }

func (l *withNewMessage) Format(s fmt.State, verb rune) { errbase.FormatError(l, s, verb) }
func (l *withNewMessage) SafeFormatError(p errbase.Printer) (next error) {
	p.Print(l.message)

	return nil /* nil here overrides the cause's message */
}

func (l *withNewMessage) SafeDetails() []string {
	return []string{l.message}
}

// leafError is like the basic error string in the stdlib
// https://go.dev/src/errors/errors.go
type leafError struct {
	msg string
}

var (
	_ error         = (*leafError)(nil)
	_ fmt.Formatter = (*leafError)(nil)
)

func (l *leafError) Error() string                 { return l.msg }
func (l *leafError) Format(s fmt.State, verb rune) { errbase.FormatError(l, s, verb) }
func (l *leafError) SafeFormatError(p errbase.Printer) (next error) {
	p.Print(l.msg)

	return nil
}

func (l *leafError) SafeDetails() []string {
	return []string{l.msg}
}
