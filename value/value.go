// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"math/big"
	"strings"

	"code.google.com/p/rspace/ivy/config"
)

var conf *config.Config

func SetConfig(c *config.Config) {
	conf = c
}

type Expr interface {
	String() string

	Eval() Value
}

type Value interface {
	String() string
	Eval() Value

	ToType(valueType) Value

	// The fmt package looks for Formatter before Stringer, but we want
	// to use Stringer only. big.Int and big.Rat implement Formatter,
	// and we embed them in our BigInt and BigRat types. To make sure
	// that our String gets called rather than the inner Format, we
	// put a non-matching stub Format method into this interface.
	// This is ugly but very simple and cheap.
	Format()
}

type Error string

func (err Error) Error() string {
	return string(err)
}

func Errorf(format string, args ...interface{}) Error {
	return Error(fmt.Sprintf(format, args...))
}

type ParseState int

func ValueString(s string) (Value, error) {
	// Is it a rational? If so, it's tricky.
	if strings.ContainsRune(s, '/') {
		elems := strings.Split(s, "/")
		if len(elems) != 2 {
			panic("bad rat")
		}
		num, err := ValueString(elems[0])
		if err != nil {
			return nil, err
		}
		den, err := ValueString(elems[1])
		if err != nil {
			return nil, err
		}
		// Common simple case.
		if whichType(num) == intType && whichType(den) == intType {
			return bigRatTwoInt64s(int64(num.(Int)), int64(den.(Int))).shrink(), nil
		}
		// General mix-em-up.
		rden := den.ToType(bigRatType)
		if rden.(BigRat).Sign() == 0 {
			panic(Error("zero denominator in rational"))
		}
		return binaryBigRatOp(num.ToType(bigRatType), (*big.Rat).Quo, rden), nil
	}
	// Not a rational, but might be something like 1.3e-2 and therefore
	// become a rational.
	i, err := SetIntString(s)
	if err == nil {
		return i, nil
	}
	b, err := SetBigIntString(s)
	if err == nil {
		return b.shrink(), nil
	}
	r, err := SetBigRatString(s)
	if err == nil {
		return r.shrink(), nil
	}
	return nil, err
}

func bigInt64(x int64) BigInt {
	return BigInt{big.NewInt(x)}
}

func bigRatInt64(x int64) BigRat {
	return bigRatTwoInt64s(x, 1)
}

func bigRatTwoInt64s(x, y int64) BigRat {
	if y == 0 {
		panic(Error("zero denominator in rational"))
	}
	return BigRat{big.NewRat(x, y)}
}
