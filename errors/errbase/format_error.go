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

// This file is forked and modified from golang.org/x/xerrors,
// at commit 3ee3066db522c6628d440a3a91c4abdd7f5ef22f (2019-05-10).
// From the original code:
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Changes specific to this fork marked as inline comments.

package errbase

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/kr/pretty"
	pkgErr "github.com/pkg/errors"
)

// FormatError formats an error according to s and verb.
// This is a helper meant for use when implementing the fmt.Formatter
// interface on custom error objects.
//
// If the error implements errors.Formatter, FormatError calls its
// FormatError method of f with an errors.Printer configured according
// to s and verb, and writes the result to s.
//
// Otherwise, if it is a wrapper, FormatError prints out its error prefix,
// then recurses on its cause.
//
// Otherwise, its Error() text is printed.
func FormatError(err error, s fmt.State, verb rune) {
	formatErrorInternal(err, s, verb)
}

// Formattable wraps an error into a fmt.Formatter which
// will provide "smart" formatting even if the outer layer
// of the error does not implement the Formatter interface.
func Formattable(err error) fmt.Formatter {
	return &errorFormatter{err}
}

// formatErrorInternal is the shared logic between FormatError
// and FormatErrorRedactable.
//
// When the redactableOutput argument is true, the fmt.State argument
// is really a redact.SafePrinter and casted down as necessary.
//
// If verb and flags are not one of the supported error formatting
// combinations (in particular, %q, %#v etc), then the redactableOutput
// argument is ignored. This limitation may be lifted in a later
// version.
func formatErrorInternal(err error, s fmt.State, verb rune) {
	// Assuming this function is only called from the Format method, and
	// given that FormatError takes precedence over Format, it cannot be
	// called from any package that supports errors.Formatter. It is
	// therefore safe to disregard that State may be a specific printer
	// implementation and use one of our choice instead.

	p := state{State: s}

	switch {
	case verb == 'v' && s.Flag('+') && !s.Flag('#'):
		// Here we are going to format as per %+v, into p.buf.
		//
		// We need to start with the innermost (root cause) error first,
		// then the layers of wrapping from innermost to outermost, so as
		// to enable stack trace de-duplication. This requires a
		// post-order traversal. Since we have a linked list, the best we
		// can do is a recursion.
		p.formatRecursive(err, true /* isOutermost */, true /* withDetail */)

		// We now have all the data, we can render the result.
		p.formatEntries(err)

		// We're done formatting. Apply width/precision parameters.
		p.finishDisplay(verb)

	case verb == 'v' && s.Flag('#'):
		// We only know how to process %#v if redactable output is not
		// requested. This is because the structured output may emit
		// arbitrary unsafe strings without redaction markers,
		// or improperly balanced/escaped redaction markers.
		if stringer, ok := err.(fmt.GoStringer); ok {
			io.WriteString(&p.finalBuf, stringer.GoString())
		} else {
			// Not a GoStringer: delegate to the pretty library.
			fmt.Fprintf(&p.finalBuf, "%#v", pretty.Formatter(err))
		}
		p.finishDisplay(verb)

	case verb == 's' ||
		// We only handle %v/%+v or other combinations here; %#v is
		// unsupported.
		(verb == 'v' && !s.Flag('#')) ||
		// We also
		// know how to format %x/%X (print bytes of error message in hex)
		// and %q (quote the result).
		(verb == 'x' || verb == 'X' || verb == 'q'):
		// Only the error message.
		//
		// Use an intermediate buffer because there may be alignment
		// instructions to obey in the final rendering or
		// quotes to add (for %q).
		//
		// Conceptually, we could just do
		//       p.buf.WriteString(err.Error())
		// However we also advertise that Error() can be implemented
		// by calling FormatError(), in which case we'd get an infinite
		// recursion. So we have no choice but to peel the data
		// and then assemble the pieces ourselves.
		p.formatRecursive(err, true /* isOutermost */, false /* withDetail */)
		p.formatSingleLineOutput()
		p.finishDisplay(verb)

	default:
		// Unknown verb. Do like fmt.Printf and tell the user we're
		// confused.
		p.finalBuf.WriteString("%!")
		p.finalBuf.WriteRune(verb)
		p.finalBuf.WriteByte('(')
		switch {
		case err != nil:
			p.finalBuf.WriteString(reflect.TypeOf(err).String())
		default:
			p.finalBuf.WriteString("<nil>")
		}
		p.finalBuf.WriteByte(')')
		io.Copy(s, &p.finalBuf)
	}
}

// formatEntries reads the entries from s.entries and produces a
// detailed rendering in s.finalBuf.
func (s *state) formatEntries(err error) {
	// The first entry at the top is special. We format it as follows:
	//
	//   <complete error message>
	//   (1) <details>
	s.formatSingleLineOutput()
	s.finalBuf.WriteString("\n(1)")

	s.printEntry(s.entries[len(s.entries)-1])

	// All the entries that follow are printed as follows:
	//
	// Wraps: (N) <details>
	//
	for i, j := len(s.entries)-2, 2; i >= 0; i, j = i-1, j+1 {
		fmt.Fprintf(&s.finalBuf, "\nWraps: (%d)", j)
		entry := s.entries[i]
		s.printEntry(entry)
	}

	// At the end, we link all the (N) references to the Go type of the
	// error.
	s.finalBuf.WriteString("\nError types:")
	for i, j := len(s.entries)-1, 1; i >= 0; i, j = i-1, j+1 {
		fmt.Fprintf(&s.finalBuf, " (%d) %T", j, s.entries[i].err)
	}
}

// printEntry renders the entry given as argument
// into s.finalBuf.
//
// If s.redactableOutput is set, then s.finalBuf is to contain
// a RedactableBytes, with redaction markers. In that
// case, we must be careful to escape (or not) the entry
// depending on entry.redactable.
//
// If s.redactableOutput is unset, then we are not caring about
// redactability. In that case entry.redactable is not set
// anyway and we can pass contents through.
func (s *state) printEntry(entry formatEntry) {
	if len(entry.head) > 0 {
		if entry.head[0] != '\n' {
			s.finalBuf.WriteByte(' ')
		}
		if len(entry.head) > 0 {
			s.finalBuf.Write(entry.head)
		}
	}
	if len(entry.details) > 0 {
		if len(entry.head) == 0 {
			if entry.details[0] != '\n' {
				s.finalBuf.WriteByte(' ')
			}
		}
		s.finalBuf.Write(entry.details)
	}
	if entry.stackTrace != nil {
		s.finalBuf.WriteString("\n  -- stack trace:")
		s.finalBuf.WriteString(strings.ReplaceAll(
			fmt.Sprintf("%+v", entry.stackTrace),
			"\n", string(detailSep)))
		if entry.elidedStackTrace {
			fmt.Fprintf(&s.finalBuf, "%s[...repeated from below...]", detailSep)
		}
	}
}

// formatSingleLineOutput prints the details extracted via
// formatRecursive() through the chain of errors as if .Error() has
// been called: it only prints the non-detail parts and prints them on
// one line with ": " separators.
//
// This function is used both when FormatError() is called indirectly
// from .Error(), e.g. in:
//      (e *myType) Error() { return fmt.Sprintf("%v", e) } (e *myType)
//      Format(s fmt.State, verb rune) { errors.FormatError(s, verb, e) }
//
// and also to print the first line in the output of a %+v format.
//
// It reads from s.entries and writes to s.finalBuf.
// s.buf is left untouched.
//
// Note that if s.redactableOutput is true, s.finalBuf is to contain a
// RedactableBytes. However, we are not using the helper facilities
// from redact.SafePrinter to do this, so care should be taken below
// to properly escape markers, etc.
func (s *state) formatSingleLineOutput() {
	for i := len(s.entries) - 1; i >= 0; i-- {
		entry := &s.entries[i]
		if entry.elideShort {
			continue
		}
		if s.finalBuf.Len() > 0 && len(entry.head) > 0 {
			s.finalBuf.WriteString(": ")
		}
		if len(entry.head) == 0 {
			// shortcut, to avoid the copy below.
			continue
		}
		s.finalBuf.Write(entry.head)
	}
}

// formatRecursive performs a post-order traversal on the chain of
// errors to collect error details from innermost to outermost.
//
// It uses s.buf as an intermediate buffer to collect strings.
// It populates s.entries as a result.
// Between each layer of error, s.buf is reset.
//
// s.finalBuf is untouched. The conversion of s.entries
// to s.finalBuf is done by formatSingleLineOutput() and/or
// formatEntries().
func (s *state) formatRecursive(err error, isOutermost, withDetail bool) {
	cause := UnwrapOnce(err)
	if cause != nil {
		// Recurse first.
		s.formatRecursive(cause, false /*isOutermost*/, withDetail)
	}

	// Reinitialize the state for this stage of wrapping.
	s.wantDetail = withDetail
	s.needSpace = false
	s.needNewline = 0
	s.multiLine = false
	s.notEmpty = false
	s.hasDetail = false
	s.headBuf = nil

	seenTrace := false

	printDone := false
	for _, fn := range specialCases {
		if handled, desiredShortening := fn(err, (*printer)(s), cause == nil /* leaf */); handled {
			printDone = true
			if desiredShortening == nil {
				// The error wants to elide the short messages from inner
				// causes. Do it.
				for i := range s.entries {
					s.entries[i].elideShort = true
				}
			}

			break
		}
	}
	if !printDone {
		switch v := err.(type) {
		case Formatter:
			desiredShortening := v.FormatError((*printer)(s))
			if desiredShortening == nil {
				// The error wants to elide the short messages from inner
				// causes. Do it.
				for i := range s.entries {
					s.entries[i].elideShort = true
				}
			}

		case fmt.Formatter:
			// We can only use a fmt.Formatter when both the following
			// conditions are true:
			// - when it is the leaf error, because a fmt.Formatter
			//   on a wrapper also recurses.
			// - when it is not the outermost wrapper, because
			//   the Format() method is likely to be calling FormatError()
			//   to do its job and we want to avoid an infinite recursion.
			if !isOutermost && cause == nil {
				v.Format(s, 'v')
				if st, ok := err.(StackTraceProvider); ok {
					// This is likely a leaf error from github/pkg/errors.
					// The thing probably printed its stack trace on its own.
					seenTrace = true
					// We'll subsequently simplify stack traces in wrappers.
					s.lastStack = st.StackTrace()
				}
			} else {
				s.formatSimple(err, cause)
			}

		default:
			// If the error did not implement errors.Formatter nor
			// fmt.Formatter, but it is a wrapper, still attempt best effort:
			// print what we can at this level.
			s.formatSimple(err, cause)
		}
	}

	// Collect the result.
	entry := s.collectEntry(err)

	// If there's an embedded stack trace, also collect it.
	// This will get either a stack from pkg/errors, or ours.
	if !seenTrace {
		if st, ok := err.(StackTraceProvider); ok {
			entry.stackTrace, entry.elidedStackTrace = ElideSharedStackTraceSuffix(
				s.lastStack,
				st.StackTrace(),
			)
			s.lastStack = entry.stackTrace
		}
	}

	// Remember the entry for later rendering.
	s.entries = append(s.entries, entry)
	s.buf = bytes.Buffer{}
}

func (s *state) collectEntry(err error) formatEntry {
	entry := formatEntry{err: err}
	if s.wantDetail {
		// The buffer has been populated as a result of formatting with
		// %+v. In that case, if the printer has separated detail
		// from non-detail, we can use the split.
		if s.hasDetail {
			entry.head = s.headBuf
			entry.details = s.buf.Bytes()
		} else {
			entry.head = s.buf.Bytes()
		}
	} else {
		entry.head = s.headBuf
		if len(entry.head) > 0 && entry.head[len(entry.head)-1] != '\n' &&
			s.buf.Len() > 0 && s.buf.Bytes()[0] != '\n' {
			entry.head = append(entry.head, '\n')
		}
		entry.head = append(entry.head, s.buf.Bytes()...)
	}

	return entry
}

// safeErrorPrinterFn is the type of a function that can take
// over the safe printing of an error. This is used to inject special
// cases into the formatting in errutil. We need this machinery to
// prevent import cycles.
type safeErrorPrinterFn = func(err error, p Printer, isLeaf bool) (handled bool, next error)

// specialCases is a list of functions to apply for special cases.
var specialCases []safeErrorPrinterFn

// RegisterSpecialCasePrinter registers a handler.
func RegisterSpecialCasePrinter(fn safeErrorPrinterFn) {
	specialCases = append(specialCases, fn)
}

// formatSimple performs a best effort at extracting the details at a
// given level of wrapping when the error object does not implement
// the Formatter interface.
func (s *state) formatSimple(err, cause error) {
	var pref string
	if cause != nil {
		pref = extractPrefix(err, cause)
	} else {
		pref = err.Error()
	}
	if len(pref) > 0 {
		s.Write([]byte(pref))
	}
}

// extractPrefix extracts the prefix from a wrapper's error message.
// For example,
//    err := errors.New("bar")
//    err = errors.Wrap(err, "foo")
//    extractPrefix(err)
// returns "foo".
func extractPrefix(err, cause error) string {
	causeSuffix := cause.Error()
	errMsg := err.Error()

	if strings.HasSuffix(errMsg, causeSuffix) {
		prefix := errMsg[:len(errMsg)-len(causeSuffix)]
		if strings.HasSuffix(prefix, ": ") {
			return prefix[:len(prefix)-2]
		}
	}

	return ""
}

// finishDisplay renders s.finalBuf into s.State.
func (p *state) finishDisplay(verb rune) {
	// Not redactable: render depending on flags and verb.

	width, okW := p.Width()
	_, okP := p.Precision()

	// If `direct` is set to false, then the buffer is always
	// passed through fmt.Printf regardless of the width and alignment
	// settings. This is important for e.g. %q where quotes must be added
	// in any case.
	// If `direct` is set to true, then the detour via
	// fmt.Printf only occurs if there is a width or alignment
	// specifier.
	direct := verb == 'v' || verb == 's'

	if !direct || (okW && width > 0) || okP {
		_, format := MakeFormat(p, verb)
		fmt.Fprintf(p.State, format, p.finalBuf.String())
	} else {
		io.Copy(p.State, &p.finalBuf)
	}
}

var detailSep = []byte("\n  | ")

// state tracks error printing state. It implements fmt.State.
type state struct {
	fmt.State
	entries                    []formatEntry
	headBuf                    []byte
	lastStack                  StackTrace
	finalBuf                   bytes.Buffer
	buf                        bytes.Buffer
	needNewline                int
	hasDetail                  bool
	collectingRedactableString bool
	notEmpty                   bool
	needSpace                  bool
	wantDetail                 bool
	multiLine                  bool
}

// formatEntry collects the textual details about one level of
// wrapping or the leaf error in an error chain.
type formatEntry struct {
	err              error
	head             []byte
	details          []byte
	stackTrace       StackTrace
	elideShort       bool
	elidedStackTrace bool
}

// String is used for debugging only.
func (e formatEntry) String() string {
	return fmt.Sprintf("formatEntry{%T, %q, %q}", e.err, e.head, e.details)
}

// Write implements io.Writer.
func (s *state) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	k := 0

	sep := detailSep
	if !s.wantDetail {
		sep = []byte("\n")
	}

	for i, c := range b {
		if c == '\n' {
			// Flush all the bytes seen so far.
			s.buf.Write(b[k:i])
			// Don't print the newline itself; instead, prepare the state so
			// that the _next_ character encountered will pad with a newline.
			// This algorithm avoids terminating error details with excess
			// newline characters.
			k = i + 1
			s.needNewline++
			s.needSpace = false
			s.multiLine = true
			if s.wantDetail {
				s.switchOver()
			}
		} else {
			if s.needNewline > 0 && s.notEmpty {
				// If newline chars were pending, display them now.
				for i := 0; i < s.needNewline-1; i++ {
					s.buf.Write(detailSep[:len(sep)-1])
				}
				s.buf.Write(sep)
				s.needNewline = 0
				s.needSpace = false
			} else if s.needSpace {
				s.buf.WriteByte(' ')
				s.needSpace = false
			}
			s.notEmpty = true
		}
	}
	s.buf.Write(b[k:])

	return len(b), nil
}

// printer wraps a state to implement an xerrors.Printer.
type printer state

func (p *state) detail() bool {
	if !p.wantDetail {
		return false
	}
	if p.notEmpty {
		p.needNewline = 1
	}
	p.switchOver()

	return true
}

func (p *state) switchOver() {
	if p.hasDetail {
		return
	}
	p.headBuf = p.buf.Bytes()
	p.buf = bytes.Buffer{}
	p.notEmpty = false
	p.hasDetail = true
}

func (s *printer) Detail() bool {
	return ((*state)(s)).detail()
}

func (s *printer) Print(args ...interface{}) {
	s.enhanceArgs(args)
	fmt.Fprint((*state)(s), args...)
}

func (s *printer) Printf(format string, args ...interface{}) {
	s.enhanceArgs(args)
	fmt.Fprintf((*state)(s), format, args...)
}

func (s *printer) enhanceArgs(args []interface{}) {
	prevStack := s.lastStack
	lastSeen := prevStack
	for i := range args {
		if st, ok := args[i].(pkgErr.StackTrace); ok {
			args[i], _ = ElideSharedStackTraceSuffix(prevStack, st)
			lastSeen = st
		}
		if err, ok := args[i].(error); ok {
			args[i] = &errorFormatter{err}
		}
	}
	s.lastStack = lastSeen
}

type errorFormatter struct{ err error }

// Format implements the fmt.Formatter interface.
func (ef *errorFormatter) Format(s fmt.State, verb rune) { FormatError(ef.err, s, verb) }

// Error implements error, so that `redact` knows what to do with it.
func (ef *errorFormatter) Error() string { return ef.err.Error() }

// Unwrap makes it a wrapper.
func (ef *errorFormatter) Unwrap() error { return ef.err }

// Cause makes it a wrapper.
func (ef *errorFormatter) Cause() error { return ef.err }

// ElideSharedStackTraceSuffix removes the suffix of newStack that's already
// present in prevStack. The function returns true if some entries
// were elided.
func ElideSharedStackTraceSuffix(prevStack, newStack StackTrace) (StackTrace, bool) {
	if len(prevStack) == 0 {
		return newStack, false
	}
	if len(newStack) == 0 {
		return newStack, false
	}

	// Skip over the common suffix.
	var i, j int
	for i, j = len(newStack)-1, len(prevStack)-1; i > 0 && j > 0; i, j = i-1, j-1 {
		if newStack[i] != prevStack[j] {
			break
		}
	}
	if i == 0 {
		// Keep at least one entry.
		i = 1
	}

	return newStack[:i], i < len(newStack)-1
}

// StackTrace is the type of the data for a call stack.
// This mirrors the type of the same name in github.com/pkg/errors.
type StackTrace = pkgErr.StackTrace // type StackTrace []Frame

// StackFrame is the type of a single call frame entry.
// This mirrors the type of the same name in github.com/pkg/errors.
type StackFrame = pkgErr.Frame // type Frame uintptr

// StackTraceProvider is a provider of StackTraces.
// This is, intentionally, defined to be implemented by pkg/errors.stack.
type StackTraceProvider interface {
	StackTrace() StackTrace
}

// MakeFormat reproduces the format currently active
// in fmt.State and verb. This is provided because Go's standard
// fmt.State does not make the original format string available to us.
//
// If the return value justV is true, then the current state
// was found to be %v exactly; in that case the caller
// can avoid a full-blown Printf call and use just Print instead
// to take a shortcut.
func MakeFormat(s fmt.State, verb rune) (justV bool, format string) {
	plus, minus, hash, sp, z := s.Flag('+'), s.Flag('-'), s.Flag('#'), s.Flag(' '), s.Flag('0')
	w, wp := s.Width()
	p, pp := s.Precision()

	if !plus && !minus && !hash && !sp && !z && !wp && !pp {
		switch verb {
		case 'v':
			return true, "%v"
		case 's':
			return false, "%s"
		case 'd':
			return false, "%d"
		}
		// Other cases handled in the slow path below.
	}

	var f strings.Builder
	f.WriteByte('%')
	if plus {
		f.WriteByte('+')
	}
	if minus {
		f.WriteByte('-')
	}
	if hash {
		f.WriteByte('#')
	}
	if sp {
		f.WriteByte(' ')
	}
	if z {
		f.WriteByte('0')
	}
	if wp {
		f.WriteString(strconv.Itoa(w))
	}
	if pp {
		f.WriteByte('.')
		f.WriteString(strconv.Itoa(p))
	}
	f.WriteRune(verb)

	return false, f.String()
}
