// Package types defines a small type lattice for pypls and the rules for
// combining and comparing types during local inference. The lattice is
// deliberately coarse. When a type cannot be determined it is Unknown, and
// Unknown is treated as compatible with everything so that inference stays
// silent rather than guessing.
package types

import "strings"

// Kind is the top-level category of a type.
type Kind int

const (
	Unknown Kind = iota
	None
	Bool
	Int
	Float
	Complex
	Str
	Bytes
	Ellipsis
	List
	Set
	Dict
	Tuple
	Callable
)

// Type is a value type. Sub-parts are best-effort and may be nil or Unknown.
type Type struct {
	Kind  Kind
	Elem  *Type   // element type for List and Set
	Key   *Type   // key type for Dict
	Value *Type   // value type for Dict
	Elems []*Type // element types for Tuple
	Name  string  // optional label for Callable
}

// Any is the shared Unknown type.
var Any = &Type{Kind: Unknown}

// Basic returns a shared type for a scalar kind.
func Basic(k Kind) *Type { return &Type{Kind: k} }

// IsUnknown reports whether the type is unknown.
func (t *Type) IsUnknown() bool { return t == nil || t.Kind == Unknown }

// IsNumeric reports whether the type is one of the numeric kinds.
func (t *Type) IsNumeric() bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case Bool, Int, Float, Complex:
		return true
	}
	return false
}

// numericRank orders the numeric kinds for widening.
func numericRank(k Kind) int {
	switch k {
	case Bool:
		return 0
	case Int:
		return 1
	case Float:
		return 2
	case Complex:
		return 3
	}
	return -1
}

// Widen returns the numeric type that results from an arithmetic operation on
// two numeric operands. Two booleans widen to int, matching Python.
func Widen(a, b *Type) *Type {
	ra, rb := numericRank(a.Kind), numericRank(b.Kind)
	hi := ra
	if rb > hi {
		hi = rb
	}
	switch hi {
	case 0: // both bool
		return Basic(Int)
	case 1:
		return Basic(Int)
	case 2:
		return Basic(Float)
	case 3:
		return Basic(Complex)
	}
	return Any
}

// String renders the type using Python-like names. Unknown renders as Any.
func (t *Type) String() string {
	if t == nil {
		return "Any"
	}
	switch t.Kind {
	case Unknown:
		return "Any"
	case None:
		return "None"
	case Bool:
		return "bool"
	case Int:
		return "int"
	case Float:
		return "float"
	case Complex:
		return "complex"
	case Str:
		return "str"
	case Bytes:
		return "bytes"
	case Ellipsis:
		return "ellipsis"
	case List:
		if t.Elem != nil && !t.Elem.IsUnknown() {
			return "list[" + t.Elem.String() + "]"
		}
		return "list"
	case Set:
		if t.Elem != nil && !t.Elem.IsUnknown() {
			return "set[" + t.Elem.String() + "]"
		}
		return "set"
	case Dict:
		if t.Key != nil && t.Value != nil && !t.Key.IsUnknown() && !t.Value.IsUnknown() {
			return "dict[" + t.Key.String() + ", " + t.Value.String() + "]"
		}
		return "dict"
	case Tuple:
		if len(t.Elems) > 0 {
			parts := make([]string, len(t.Elems))
			for i, e := range t.Elems {
				parts[i] = e.String()
			}
			return "tuple[" + strings.Join(parts, ", ") + "]"
		}
		return "tuple"
	case Callable:
		if t.Name != "" {
			return t.Name
		}
		return "callable"
	}
	return "Any"
}

// Assignable reports whether a value of type v can be assigned where type target
// is expected. Unknown on either side is always assignable, which keeps the
// checker quiet on code it cannot fully understand. None is accepted everywhere
// so that optional patterns do not produce noise.
func Assignable(v, target *Type) bool {
	if v.IsUnknown() || target.IsUnknown() {
		return true
	}
	if v.Kind == None || target.Kind == None {
		return true
	}
	// Numeric widening: a lower-ranked numeric fits a higher-ranked one.
	if v.IsNumeric() && target.IsNumeric() {
		return numericRank(v.Kind) <= numericRank(target.Kind)
	}
	// Otherwise the top-level kind must match. Element types are not compared,
	// to avoid false positives on partially known containers.
	return v.Kind == target.Kind
}

// Join returns a common type for two values, used for container elements. When
// the two disagree the result is Unknown.
func Join(a, b *Type) *Type {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Kind == b.Kind {
		return a
	}
	if a.IsNumeric() && b.IsNumeric() {
		return Widen(a, b)
	}
	return Any
}
