// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

// Binary operators.

// To aovid initialization cycles when we refer to the ops from inside
// themselves, we use an init function to initialize the ops.

// binaryArithType returns the maximum of the two types,
// so the smaller value is appropriately up-converted.
func binaryArithType(t1, t2 valueType) valueType {
	if t1 > t2 {
		return t1
	}
	return t2
}

// divType is like binaryArithType but never returns smaller than BigInt,
// because the only implementation of exponentiation we have is in big.Int.
func divType(t1, t2 valueType) valueType {
	if t1 == intType {
		t1 = bigIntType
	}
	return binaryArithType(t1, t2)
}

// rationalType promotes scalars to rationals so we can do rational division.
func rationalType(t1, t2 valueType) valueType {
	if t1 < bigRatType {
		t1 = bigRatType
	}
	return binaryArithType(t1, t2)
}

// shiftCount converts x to an unsigned integer.
func shiftCount(x Value) uint {
	switch count := x.(type) {
	case Int:
		if count.x < 0 || count.x >= maxInt {
			panic(Errorf("illegal shift count %d", count.x))
		}
		return uint(count.x)
	case BigInt:
		// Must be small enough for an int; that will happen if
		// the LHS is a BigInt because the RHS will have been lifted.
		reduced := count.shrink()
		if _, ok := reduced.(Int); ok {
			return shiftCount(reduced)
		}
	}
	panic(Error("illegal shift count type"))
}

// binaryVectorOp applies op elementwise to i and j.
func binaryVectorOp(i Value, op string, j Value) Value {
	u, v := i.(Vector), j.(Vector)
	if len(u.x) == 1 {
		n := make([]Value, v.Len())
		for k := range v.x {
			n[k] = Binary(u.x[0], op, v.x[k])
		}
		return ValueSlice(n)
	}
	if len(v.x) == 1 {
		n := make([]Value, u.Len())
		for k := range u.x {
			n[k] = Binary(u.x[k], op, v.x[0])
		}
		return ValueSlice(n)
	}
	u.sameLength(v)
	n := make([]Value, u.Len())
	for k := range u.x {
		n[k] = Binary(u.x[k], op, v.x[k])
	}
	return ValueSlice(n)
}

func binaryBigIntOp(u Value, op func(*big.Int, *big.Int, *big.Int) *big.Int, v Value) Value {
	i, j := u.(BigInt), v.(BigInt)
	z := bigInt64(0)
	op(z.x, i.x, j.x)
	return z.shrink()
}

func binaryBigRatOp(u Value, op func(*big.Rat, *big.Rat, *big.Rat) *big.Rat, v Value) Value {
	i, j := u.(BigRat), v.(BigRat)
	z := bigRatInt64(0)
	op(z.x, i.x, j.x)
	return z.shrink()
}

// bigIntPow is the "op" for pow on *big.Int. Different signature for Exp means we can't use *big.Exp directly.
func bigIntPow(i, j, k *big.Int) *big.Int {
	i.Exp(j, k, nil)
	return i
}

// toInt turns the boolean into 0 or 1.
func toInt(t bool) Value {
	if t {
		return one
	}
	return zero
}

var (
	add, sub, mul, pow        *binaryOp
	quo, idiv, imod, div, mod *binaryOp
	and, or, xor, lsh, rsh    *binaryOp
	eq, ne, lt, le, gt, ge    *binaryOp
	min, max                  *binaryOp
	binaryOps                 map[string]*binaryOp
)

var (
	zero        = valueInt64(0)
	one         = valueInt64(1)
	minusOne    = valueInt64(-1)
	bigZero     = bigInt64(0)
	bigOne      = bigInt64(1)
	bigMinusOne = bigInt64(-1)
)

func init() {
	add = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return valueInt64(u.(Int).x + v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Add, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Add, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "+", v)
			},
		},
	}

	sub = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return valueInt64(u.(Int).x - v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Sub, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Sub, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "-", v)
			},
		},
	}

	mul = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return valueInt64(u.(Int).x * v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Mul, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Mul, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "*", v)
			},
		},
	}

	quo = &binaryOp{ // Rational division.
		whichType: rationalType, // Use BigRats to avoid the analysis here.
		fn: [numType]binaryFn{
			bigRatType: func(u, v Value) Value {
				x := v.(BigRat)
				if x.x.Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigRatOp(u, (*big.Rat).Quo, v) // True division.
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "/", v)
			},
		},
	}

	idiv = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				if v.(Int).x == 0 {
					panic(Error("division by zero"))
				}
				return valueInt64(u.(Int).x / v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				x := v.(BigInt)
				if x.x.Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Quo, v) // Go-like division.
			},
			bigRatType: nil, // Not defined for rationals. Use div.
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "idiv", v)
			},
		},
	}

	imod = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				if v.(Int).x == 0 {
					panic(Error("modulo by zero"))
				}
				return valueInt64(u.(Int).x % v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				x := v.(BigInt)
				if x.x.Sign() == 0 {
					panic(Error("modulo by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Rem, v) // Go-like modulo.
			},
			bigRatType: nil, // Not defined for rationals. Use mod.
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "imod", v)
			},
		},
	}

	div = &binaryOp{ // Euclidean integer division.
		whichType: divType, // Use BigInts to avoid the analysis here.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				x := v.(BigInt)
				if x.x.Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Div, v) // Euclidean division.
			},
			bigRatType: nil, // Not defined for rationals. Use div.
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "div", v)
			},
		},
	}

	mod = &binaryOp{ // Euclidean integer modulus.
		whichType: divType, // Use BigInts to avoid the analysis here.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				x := v.(BigInt)
				if x.x.Sign() == 0 {
					panic(Error("modulo by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Mod, v) // Euclidan modulo.
			},
			bigRatType: nil, // Not defined for rationals. Use mod.
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "mod", v)
			},
		},
	}

	pow = &binaryOp{
		whichType: divType,
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				i := v.(BigInt)
				switch i.x.Sign() {
				case 0:
					return one
				case -1:
					panic(Error("negative exponent not implemented"))
				}
				return binaryBigIntOp(u, bigIntPow, v)
			},
			bigRatType: func(u, v Value) Value {
				// We know v is integral. (n/d)**2 is n**2/d**2.
				rexp := v.(BigRat).x
				switch rexp.Sign() {
				case 0:
					return one
				case -1:
					panic(Error("negative exponent not implemented"))
				}
				exp := rexp.Num()
				rat := u.(BigRat).x
				num := rat.Num()
				den := rat.Denom()
				num.Exp(num, exp, nil)
				den.Exp(den, exp, nil)
				z := bigRatInt64(0)
				z.x.SetFrac(num, den)
				return z
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "**", v)
			},
		},
	}

	and = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return valueInt64(u.(Int).x & v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).And, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "&", v)
			},
		},
	}

	or = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return valueInt64(u.(Int).x | v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Or, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "|", v)
			},
		},
	}

	xor = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return valueInt64(u.(Int).x ^ v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Xor, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "^", v)
			},
		},
	}

	lsh = &binaryOp{
		whichType: divType, // Shifts are like power: let BigInt do the work.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				z := bigInt64(0)
				z.x.Lsh(i.x, shiftCount(j))
				return z.shrink()
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "<<", v)
			},
		},
	}

	rsh = &binaryOp{
		whichType: divType, // Shifts are like power: let BigInt do the work.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				z := bigInt64(0)
				z.x.Rsh(i.x, shiftCount(j))
				return z.shrink()
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, ">>", v)
			},
		},
	}

	eq = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int).x == v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(j.x) == 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(j.x) == 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "==", v)
			},
		},
	}

	ne = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int).x != v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(j.x) != 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(j.x) != 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "!=", v)
			},
		},
	}

	lt = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int).x < v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(j.x) < 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(j.x) < 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "<", v)
			},
		},
	}

	le = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int).x <= v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(j.x) <= 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(j.x) <= 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "<=", v)
			},
		},
	}

	gt = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int).x > v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(j.x) > 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(j.x) > 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, ">", v)
			},
		},
	}

	ge = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int).x >= v.(Int).x)
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(j.x) >= 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(j.x) >= 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, ">=", v)
			},
		},
	}

	min = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				i, j := u.(Int).x, v.(Int).x
				if i < j {
					return u
				}
				return v
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				if i.x.Cmp(j.x) < 0 {
					return u
				}
				return v
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				if i.x.Cmp(j.x) < 0 {
					return u
				}
				return v
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "min", v)
			},
		},
	}

	max = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				i, j := u.(Int).x, v.(Int).x
				if i > j {
					return u
				}
				return v
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				if i.x.Cmp(j.x) > 0 {
					return u
				}
				return v
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				if i.x.Cmp(j.x) > 0 {
					return u
				}
				return v
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "min", v)
			},
		},
	}

	binaryOps = map[string]*binaryOp{
		"+":    add,
		"-":    sub,
		"*":    mul,
		"/":    quo,  // Exact rational division.
		"idiv": idiv, // Go-like truncating integer division.
		"imod": imod, // Go-like integer moduls.
		"div":  div,  // Euclidean integer division.
		"mod":  mod,  // Euclidean integer division.
		"**":   pow,
		"&":    and,
		"|":    or,
		"^":    xor,
		"<<":   lsh,
		">>":   rsh,
		"==":   eq,
		"!=":   ne,
		"<":    lt,
		"<=":   le,
		">":    gt,
		">=":   ge,
		"min":  min,
		"max":  max,
	}
}
