// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
)

type Vector []Value

func (v Vector) String() string {
	var b bytes.Buffer
	for i, elem := range v {
		if i > 0 {
			fmt.Fprint(&b, " ")
		}
		fmt.Fprintf(&b, "%s", elem)
	}
	return b.String()
}

func ValueSlice(elem []Value) Vector {
	return Vector(elem)
}

func (v Vector) Eval() Value {
	return v
}

func (v Vector) ToType(which valueType) Value {
	switch which {
	case intType:
		panic("bigint to int")
	case bigIntType:
		panic("vector to big int")
	case bigRatType:
		panic("vector to big rat")
	case vectorType:
		return v
	}
	panic("BigInt.ToType")
}

func (v Vector) Len() int {
	return len(v)
}

func (v Vector) sameLength(x Vector) {
	if v.Len() != x.Len() {
		panic(Errorf("length mismatch: %d %d", v.Len(), x.Len()))
	}
}
