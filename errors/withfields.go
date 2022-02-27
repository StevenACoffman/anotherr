package errors

import (
	"fmt"
	"sort"

	"github.com/StevenACoffman/anotherr/errors/errbase"
)

// WithFields is our wrapper type.
type withFields struct {
	cause  error
	fields Fields
	*stack
}

type Fields map[string]interface{}

// WrapWithFields adds fields to an existing error.
func WrapWithFields(err error, fields Fields) error {
	if err == nil {
		return nil
	}

	return WrapWithFieldsAndDepth(err, fields, 1)
}

// WrapWithFieldsAndDepth adds fields to an existing error
// and captures the stacktrace
func WrapWithFieldsAndDepth(err error, fields Fields, depth int) error {
	if err == nil {
		return nil
	}

	return &withFields{cause: err, fields: fields, stack: callers(depth + 1)}
}

// GetFields retrieves the Fields from a stack of causes.
func GetFields(err error) Fields {
	if w, ok := err.(*withFields); ok {
		return w.fields
	}

	return nil
}

// it's an error.
func (w *withFields) Error() string { return w.cause.Error() }

// Cause makes it also a wrapper.
func (w *withFields) Cause() error  { return w.cause }
func (w *withFields) Unwrap() error { return w.cause }

// Format knows how to format itself.
func (w *withFields) Format(s fmt.State, verb rune) { errbase.FormatError(w, s, verb) }

// SafeFormatError implements errors.SafeFormatter.
// Note: see the documentation of errbase.SafeFormatter for details
// on how to implement this. In particular beware of not emitting
// unsafe strings.
func (w *withFields) SafeFormatError(p errbase.Printer) (next error) {
	if p.Detail() && w.fields != nil && len(w.fields) != 0 {
		var empty string
		p.Printf("fields: [")

		keys := make([]string, 0, len(w.fields))
		for k := range w.fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			v := w.fields[k]
			eq := empty
			var val interface{} = empty
			fmt.Println(k, w.fields[k])
			if i > 0 {
				p.Printf(",")
			}
			if v != nil {
				if len(k) > 1 {
					eq = ":"
				}
				val = v
			}

			p.Print(fmt.Sprintf("%s%s%v", k, eq, val))
		}

		p.Printf("], ")
	}

	// We do not print the stack trace ourselves - errbase.FormatError()
	// does this for us.
	return w.cause
}

func fieldsIterate(fields Fields, fn func(i int, s string)) {
	var empty string
	i := 0
	for k, v := range fields {
		eq := empty
		var val interface{} = empty
		if v != nil {
			if len(k) > 1 {
				eq = ":"
			}
			val = v
		}
		res := fmt.Sprintf("%s%s%v", k, eq, val)
		fn(i, res)
		i++
	}
}

// SafeDetails implements the errbase.SafeDetailer interface.
func (w *withFields) SafeDetails() []string {
	return []string{fmt.Sprintf("%+v", w.StackTrace())}
}

//func (w *withFields) MarshalLogObject(enc zapcore.ObjectEncoder) error {
//	enc.AddString("message", w.Error())
//	enc.AddString("stacktrace", fmt.Sprintf("%+v", w.StackTrace()))
//	err := enc.AddReflected("fields", w.fields)
//	if err != nil {
//		return errors.Wrapf(err, "Unable to AddReflected fields to log: %+v", w.fields)
//	}
//	err = enc.AddReflected("cause", w.cause)
//	if err != nil {
//		return errors.Wrapf(err, "Unable to AddReflected cause to log %+v", w.cause)
//	}
//
//	return nil
//}
