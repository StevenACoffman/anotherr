This package aims to be used as a drop-in replacement to github.com/pkg/errors and Go's standard errors package.

Compatibility with Khan webapp errors is also provided, but instead of Wrap, use KhanWrap.
All other functions, like `NotFound(args ...interface{})` should be drop-in replacements.

Additionally, it provides all the benefits of cockroachdb/errors:

- it provides `Wrap` primitives akin to those found in
  `github.com/pkg/errors`.
- it is compatible with both the `causer` interface (`Cause() error`) from
  `github.com/pkg/errors` and the `Wrapper` interface (`Unwrap() error`) from Go 2.
- it enables fast, reliable and secure determination of whether
  a particular cause is present (not relying on the presence of a substring in the error message).
- [Stack traces for troubleshooting](https://github.com/cockroachdb/cockroach/blob/master/docs/RFCS/20190318_error_handling.md#Stack-traces-for-troubleshooting)
- it is composable, which makes it extensible with additional error annotations;
  for example, the basic functionality has HTTP error codes.
