// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package errorutil

import (
	"errors"
	"path"
	"runtime"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func getLine() int {
	_, _, line, ok := runtime.Caller(1)

	if !ok {
		panic("Could not get line number on test")
	}

	return line
}

type customError struct {
	answer int
	inner  error
}

func (e *customError) Chain() ErrorChain {
	return BuildChain(e, e.inner)
}

func (e *customError) Error() string {
	return "custom_error"
}

func (e *customError) Unwrap() error {
	return e.inner
}

type customWrappingError struct {
	inner error
}

func (e *customWrappingError) Error() string {
	return "another_custom_error"
}

func (e *customWrappingError) Unwrap() error {
	return e.inner
}

func TestErrorWrapping(t *testing.T) {
	Convey("Empty message", t, func() {
		err := errors.New("Boom")
		w, line := Wrap(err), getLine()
		So(errors.Is(w, err), ShouldBeTrue)
		So(path.Base(w.Filename), ShouldEqual, "errors_test.go")
		So(w.Line, ShouldEqual, line)
		So(w.Msg, ShouldEqual, "")
	})

	Convey("Non empty message", t, func() {
		err := errors.New("Boom")
		w, line := Wrap(err, "This is the ", "Answer: ", 42), getLine()
		So(errors.Is(w, err), ShouldBeTrue)
		So(path.Base(w.Filename), ShouldEqual, "errors_test.go")
		So(w.Line, ShouldEqual, line)
		So(w.Msg, ShouldEqual, "This is the Answer: 42")
	})

	Convey("Errors chain", t, func() {
		countLines := func(c ErrorChain) int {
			msg := strings.Trim(c.Error(), "\n")
			t.Log("{", msg, "}")
			return len(strings.Split(msg, "\n"))
		}

		Convey("Only Wraps", func() {
			e1 := errors.New("e1")
			e2 := Wrap(e1, "wrapping e1")
			e3 := Wrap(e2)
			e4 := Wrap(e3)

			So(Chain(e2), ShouldResemble, ErrorChain{e2, e1})
			So(Chain(e4), ShouldResemble, ErrorChain{e4, e3, e2, e1})
			So(errors.Is(e4, e1), ShouldBeTrue)

			So(countLines(Chain(e2)), ShouldEqual, 1)
			So(countLines(Chain(e3)), ShouldEqual, 1)
			So(countLines(Chain(e4)), ShouldEqual, 1)

			Convey("Unwrap", func() {
				So(TryToUnwrap(nil), ShouldBeNil)
				So(TryToUnwrap(e1), ShouldEqual, e1)
				So(TryToUnwrap(e2), ShouldEqual, e1)
				So(TryToUnwrap(e3), ShouldEqual, e1)
				So(TryToUnwrap(e4), ShouldEqual, e1)
			})
		})

		Convey("With custom error", func() {
			e1 := errors.New("e1")
			e2 := Wrap(e1, "wrapping e1")
			e3 := &customError{42, e2}
			e4 := Wrap(e3)

			So(Chain(e4), ShouldResemble, ErrorChain{e4, e3, e2, e1})
			So(errors.Is(e4, e1), ShouldBeTrue)
			So(errors.Is(e3, e1), ShouldBeTrue)
			So(errors.Is(e3, e2), ShouldBeTrue)
			So(errors.Is(e4, e3), ShouldBeTrue)
			So(errors.Is(e3, e4), ShouldBeFalse)

			So(countLines(Chain(e3)), ShouldEqual, 1)
			So(countLines(Chain(e4)), ShouldEqual, 1)

			Convey("Unwrap", func() {
				So(TryToUnwrap(e1), ShouldEqual, e1)
				So(TryToUnwrap(e2), ShouldEqual, e1)
				So(TryToUnwrap(e3), ShouldEqual, e1)
				So(TryToUnwrap(e4), ShouldEqual, e1)
			})

			Convey("Try to cast", func() {
				{
					_, ok := ErrorAs(e4, nil)
					So(ok, ShouldBeFalse)
				}

				{
					_, ok := ErrorAs(e1, &customError{})
					So(ok, ShouldBeFalse)
				}

				{
					// Same type
					e, ok := ErrorAs(e2, &Error{})
					So(ok, ShouldBeTrue)
					_, ok = e.(*Error)
					So(ok, ShouldBeTrue)
				}

				{
					e, ok := ErrorAs(e4, &customError{})
					So(ok, ShouldBeTrue)
					_, ok = e.(*customError)
					So(ok, ShouldBeTrue)
				}

				{
					_, ok := ErrorAs(e2, &customWrappingError{})
					So(ok, ShouldBeFalse)
				}
			})
		})
	})
}
