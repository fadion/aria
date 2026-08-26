package parser

import "github.com/fadion/aria/internal/token"

// Binding powers, loosest to tightest. This table is the whole of the operator
// grammar; docs/architecture.md records why it looks like this.
//
// Three levels differ from the original parser, all of them deliberate:
//
//   - `&&` and `||` get distinct levels. They shared one before, so
//     `false && false || true` grouped as `false && (false || true)` and
//     evaluated to false where every other language gives true.
//   - `**` is right-associative, so `2 ** 3 ** 2` is 512 rather than 64.
//   - Bitwise binds tighter than comparison, so `6 & 3 == 3` is `(6 & 3) == 3`
//     rather than the C footgun `6 & (3 == 3)`, which was a type error. This is
//     Python's ordering: comparison sits *looser* than `|` and `&`.
const (
	lowest     = iota
	precAssign // = += -= *= /=
	precPipe   // |>
	precArrow  // ->
	precTernary
	precOr         // ||
	precAnd        // &&
	precComparison // == != < <= > >=
	precBitOr      // |
	precBitAnd     // &
	precRange      // ..
	precShift      // << >>
	precSum        // + -
	precProduct    // * / %
	precPower      // **
	precPrefix     // ! ~ -
	precCall       // ( [ .
	precTypeOp     // is as
)

// leftBindingPower is how tightly an infix or postfix operator pulls on the
// expression to its left. An operator that cannot appear in infix position has
// no entry and stops expression parsing.
var leftBindingPower = [...]int{
	token.Assign:      precAssign,
	token.AssignPlus:  precAssign,
	token.AssignMinus: precAssign,
	token.AssignMul:   precAssign,
	token.AssignDiv:   precAssign,

	token.Pipe:     precPipe,
	token.Arrow:    precArrow,
	token.Question: precTernary,

	token.Or:  precOr,
	token.And: precAnd,

	token.BitOr:  precBitOr,
	token.BitAnd: precBitAnd,

	token.Eq:    precComparison,
	token.NotEq: precComparison,
	token.Lt:    precComparison,
	token.LtEq:  precComparison,
	token.Gt:    precComparison,
	token.GtEq:  precComparison,

	token.Range: precRange,

	token.ShiftLeft:  precShift,
	token.ShiftRight: precShift,

	token.Plus:  precSum,
	token.Minus: precSum,

	token.Star:    precProduct,
	token.Slash:   precProduct,
	token.Percent: precProduct,

	token.Power: precPower,

	token.LParen:   precCall,
	token.LBracket: precCall,
	token.Dot:      precCall,

	token.Is: precTypeOp,
	token.As: precTypeOp,
}

// lbp returns the left binding power of kind, or lowest if it does not bind.
func lbp(kind token.Kind) int {
	if int(kind) < len(leftBindingPower) {
		return leftBindingPower[kind]
	}
	return lowest
}

// rightBindingPower is the precedence an infix operator parses its right side
// at. For a left-associative operator that is its own binding power, so an
// operator of equal precedence to the right stops and becomes a sibling. For a
// right-associative one it is one lower, letting an equal operator nest inside.
func rightBindingPower(kind token.Kind) int {
	switch kind {
	case token.Power:
		// Right-associative: 2 ** 3 ** 2 is 2 ** (3 ** 2).
		return precPower - 1
	case token.Or, token.And:
		// Left-associative, so a && b && c is (a && b) && c.
		return lbp(kind)
	}
	return lbp(kind)
}

// prefixBindingPower is the precedence a prefix operator parses its operand at.
//
// It is deliberately below precPower rather than at precPrefix, so `**` wins
// against a prefix operator to its left while a prefix operator may still open
// the exponent:
//
//	-2 ** 2  ->  -(2 ** 2)  =  -4
//	2 ** -1  ->  2 ** (-1)  =   0
//
// This is Python's rule. The original parser gave prefix the higher power, so
// `-2 ** 2` grouped as `(-2) ** 2` and came out 4.
const prefixBindingPower = precPower - 1
