package main

import (
	"errors"
	"math/big"
)

// StableSwap math for a pool initialized with curve "stable".
//
// A COPY of go-vsc-node modules/incentive-pendulum/stableswap.go (all of it
// but the node-only StableMarginalPrice and SlipFee): the node prices the
// swap with these functions and this contract checks the result with the
// same ones, so the two must stay identical (same divisions, same order,
// same stop rule).
//
// Two assets, Curve's integer form (Ann = A·2):
//
//	Ann·(x + y) + D = Ann·D + D³ / (4·x·y)
//
// Reserves are normalized to 18 decimals (StableRate) before use.

// StableMaxIterations caps both Newton loops. Curve uses the same cap; the
// loops converge in a handful of steps for any sane reserves.
const StableMaxIterations = 255

// StableMaxAmp bounds the amplification coefficient A.
const StableMaxAmp int64 = 10_000

var (
	// ErrStableInput: a non-positive reserve or an amplification outside
	// [1, StableMaxAmp].
	ErrStableInput = errors.New("stableswap: invalid input")
	// ErrStableNoConverge: a Newton loop did not settle within
	// StableMaxIterations. The swap is refused rather than priced on an
	// unconverged value.
	ErrStableNoConverge = errors.New("stableswap: did not converge")
)

var (
	bigOne   = big.NewInt(1)
	bigTwo   = big.NewInt(2)
	bigThree = big.NewInt(3)
	bigFour  = big.NewInt(4)
)

func stableAnn(amp int64) (*big.Int, error) {
	if amp < 1 || amp > StableMaxAmp {
		return nil, ErrStableInput
	}
	return big.NewInt(amp * 2), nil
}

// StableGetD returns the invariant D for normalized reserves (x, y).
//
//	D_P    = D³ / (4·x·y)                       (one floor division)
//	D_next = (Ann·S + 2·D_P)·D / ((Ann − 1)·D + 3·D_P)
//
// starting from D = S = x + y, until |D_next − D| ≤ 1.
func StableGetD(x, y *big.Int, amp int64) (*big.Int, error) {
	if x == nil || y == nil || x.Sign() <= 0 || y.Sign() <= 0 {
		return nil, ErrStableInput
	}
	ann, err := stableAnn(amp)
	if err != nil {
		return nil, err
	}
	s := new(big.Int).Add(x, y)
	d := new(big.Int).Set(s)
	annS := new(big.Int).Mul(ann, s)
	annMinusOne := new(big.Int).Sub(ann, bigOne)
	fourXY := new(big.Int).Mul(x, y)
	fourXY.Mul(fourXY, bigFour)
	for i := 0; i < StableMaxIterations; i++ {
		// One floor division. Curve splits it (D·D/(2x)·D/(2y)) only to fit
		// uint256; the double flooring loses enough at a 1e5:1 imbalance for
		// the integer iteration to fall into a 2-cycle that never settles.
		dp := new(big.Int).Mul(d, d)
		dp.Mul(dp, d)
		dp.Quo(dp, fourXY)

		prev := new(big.Int).Set(d)
		num := new(big.Int).Mul(dp, bigTwo)
		num.Add(num, annS)
		num.Mul(num, d)
		den := new(big.Int).Mul(annMinusOne, d)
		den.Add(den, new(big.Int).Mul(bigThree, dp))
		if den.Sign() <= 0 {
			return nil, ErrStableNoConverge
		}
		d = num.Quo(num, den)
		if diff := new(big.Int).Sub(d, prev); diff.CmpAbs(bigOne) <= 0 {
			return d, nil
		}
	}
	return nil, ErrStableNoConverge
}

// StableGetY returns the other reserve that keeps invariant D when one
// reserve becomes xNew (the invariant is symmetric, so this serves either
// side). Solves y² + (b − D)·y = c with
//
//	c = D³ / (4·xNew·Ann)   (one floor division)
//	b = xNew + D/Ann
//
// by y_next = (y² + c) / (2y + b − D) from y = D, until |y_next − y| ≤ 1.
func StableGetY(xNew, d *big.Int, amp int64) (*big.Int, error) {
	if xNew == nil || d == nil || xNew.Sign() <= 0 || d.Sign() <= 0 {
		return nil, ErrStableInput
	}
	ann, err := stableAnn(amp)
	if err != nil {
		return nil, err
	}
	c := new(big.Int).Mul(d, d)
	c.Mul(c, d)
	c.Quo(c, new(big.Int).Mul(new(big.Int).Mul(xNew, ann), bigFour))
	b := new(big.Int).Quo(d, ann)
	b.Add(b, xNew)

	y := new(big.Int).Set(d)
	for i := 0; i < StableMaxIterations; i++ {
		prev := new(big.Int).Set(y)
		num := new(big.Int).Mul(y, y)
		num.Add(num, c)
		den := new(big.Int).Mul(y, bigTwo)
		den.Add(den, b)
		den.Sub(den, d)
		if den.Sign() <= 0 {
			return nil, ErrStableNoConverge
		}
		y = num.Quo(num, den)
		if diff := new(big.Int).Sub(y, prev); diff.CmpAbs(bigOne) <= 0 {
			return y, nil
		}
	}
	return nil, ErrStableNoConverge
}

// StablePrecisionDecimals is the normalized unit: 18 decimals, as Curve.
const StablePrecisionDecimals = 18

// StableRate returns 10^(18 − decimals), the multiplier that turns an asset's
// base units into the pool's normalized unit. Decimals above 18 are refused.
func StableRate(decimals uint8) (*big.Int, error) {
	if decimals > StablePrecisionDecimals {
		return nil, ErrStableInput
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(StablePrecisionDecimals-decimals)), nil), nil
}

// StableFloorY is the pool contract's swap check: the least output-side
// reserve (normalized) that keeps the invariant of the pre-swap reserves
// (X, Y) when the input-side reserve is storedX. A swap is honest iff
// storedY·rateY ≥ StableFloorY(...). All arguments in base units except the
// result.
func StableFloorY(storedX, X, Y, rateX, rateY *big.Int, amp int64) (*big.Int, error) {
	if storedX == nil || X == nil || Y == nil || rateX == nil || rateY == nil ||
		rateX.Sign() <= 0 || rateY.Sign() <= 0 {
		return nil, ErrStableInput
	}
	d0, err := StableGetD(new(big.Int).Mul(X, rateX), new(big.Int).Mul(Y, rateY), amp)
	if err != nil {
		return nil, err
	}
	return StableGetY(new(big.Int).Mul(storedX, rateX), d0, amp)
}
