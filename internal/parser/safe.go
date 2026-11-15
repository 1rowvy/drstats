package parser

import (
	"fmt"
	"runtime/debug"
)

// ParseError wraps any failure that happened while decoding a replay,
// including recovered panics from manta (which happen after Dota patches).
type ParseError struct {
	Msg   string
	Panic bool
	Stack string
}

func (e *ParseError) Error() string {
	if e.Panic {
		return "parser panic: " + e.Msg
	}
	return e.Msg
}

// Safe runs fn and converts both returned errors and panics into *ParseError,
// so a broken replay can never take the application down.
func Safe(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ParseError{Msg: fmt.Sprint(r), Panic: true, Stack: string(debug.Stack())}
		}
	}()
	if e := fn(); e != nil {
		return &ParseError{Msg: e.Error()}
	}
	return nil
}
