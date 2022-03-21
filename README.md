### anotherr - Another Go error package 
The package `github.com/StevenACoffman/anotherr/errors`aims to be used as a drop-in replacement to `github.com/pkg/errors` and Go's standard errors package,
due to it being a crudely hacked up version of [cockroachdb/errors](https://github.com/cockroachdb/errors). (see why below)

Compatibility with Khan Academy webapp errors is also provided, but instead of `github.com/Khan/webapp/pkg/lib/errors.Wrap`, use `errors.KhanWrap` for when `func Wrap(err error, msg string) error`
doesn't match the Khan style `func KhanWrap(err error, args ...interface{})` usage. All other functions, like `NotFound(args ...interface{})` should be drop-in replacements. 

Additionally, it provides **_some_** of the benefits of [cockroachdb/errors](https://github.com/cockroachdb/errors):

- it provides `Wrap` primitives akin to those found in
  `github.com/pkg/errors`.
- it is compatible with both the `causer` interface (`Cause() error`) from
  `github.com/pkg/errors` and the `Wrapper` interface (`Unwrap() error`) from Go 2.
- it enables fast, reliable and secure determination of whether
  a particular cause is present (not relying on the presence of a substring in the error message).
- [Stack traces for troubleshooting](https://github.com/cockroachdb/cockroach/blob/master/docs/RFCS/20190318_error_handling.md#Stack-traces-for-troubleshooting)
- it is composable, which makes it extensible with additional error annotations;
  for example, the basic functionality has HTTP error codes.

It does *NOT* provide:
+ comprehensive support for PII-free reportable strings
+ `errors.SafeFormatError()`, `SafeFormatter`
+ wrappers to attach logtags details from context.Context
+ transparent protobuf encode/decode with forward compatibility
+ wrappers to denote assertion failures
+ wrapper-aware `IsPermission()`, `IsTimeout()`, `IsExist()`, `IsNotExist()`

### Why Not [cockroachdb/errors](https://github.com/cockroachdb/errors)?

So [cockroachdb/errors](https://github.com/cockroachdb/errors) is wonderful in all ways, but HEAVY.

Last I checked, [cockroachdb/errors](https://github.com/cockroachdb/errors) does not work from the Go playground, although I'm not sure why.

So I very crudely hacked up a version that works in the Go playground. Mainly, this is so we can make working demos for talks.

The `_example/main.go` is to provide an example of the output.
```
$ go run main.go
error: something went wrong
(1)
  -- stack trace:
  | main.bar
  | 	/Users/steve/Documents/git/anotherr/cmd/main.go:27
  | main.main
  | 	/Users/steve/Documents/git/anotherr/cmd/main.go:37
  | runtime.main
  | 	/Users/steve/.asdf/installs/golang/1.16.12/go/src/runtime/proc.go:225
  | runtime.goexit
  | 	/Users/steve/.asdf/installs/golang/1.16.12/go/src/runtime/asm_amd64.s:1371
Wraps: (2) error
Wraps: (3) something went wrong
Error types: (1) *withstack.withStack (2) *errutil.withPrefix (3) main.ErrMyError
```