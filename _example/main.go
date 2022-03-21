package main

import (
	"fmt"

	"github.com/StevenACoffman/anotherr/errors"
)

// ErrSomethingWentWrong is a sentinel error which can be useful within a single API layer.
var ErrSomethingWentWrong = errors.New("Something went wrong")

// ErrMyError is an error that can be returned from a public API.
type ErrMyError struct {
	Msg string
}

func (e ErrMyError) Error() string {
	return e.Msg
}

func foo() error {
	// Attach stack trace to the sentinel error.
	return errors.WithStack(ErrSomethingWentWrong)
}

func bar() error {
	return errors.Wrap(ErrMyError{"something went wrong"}, "error")
}

func baz() error {
	return errors.KhanWrap(fmt.Errorf("something"), "key", "value")
}

func qux() error {
	return errors.Internal("key", "value")
}

func main() {
	fmt.Println("foo:")
	if err := foo(); err != nil {
		if errors.Is(err, ErrSomethingWentWrong) {
			fmt.Printf("Is %+v\n", err)
		} else {
			fmt.Printf("Is Not %+v\n", err)
		}
	}
	fmt.Println("\nWrap:")
	if err := bar(); err != nil {
		if errors.As(err, &ErrMyError{}) {
			fmt.Printf("%+v\n", err)
		}
	}
	fmt.Printf("\nKhan Style:\n%+v\n", baz())

	fmt.Printf("\nWrapped Khan Internal:\n%+v\n", errors.Wrap(qux(), "wrapped"))

}
