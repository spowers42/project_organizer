package core

import "math/rand"

// Rand is the only source of non-determinism in the application. It is used
// solely by the Do Next weighted pick. Tests inject an instance seeded to a
// constant so picks are reproducible. The interface grows as the pick needs
// more of math/rand's surface.
type Rand interface {
	// Float64 returns a pseudo-random number in [0.0, 1.0).
	Float64() float64
}

// NewRand returns a Rand backed by math/rand, seeded deterministically.
func NewRand(seed int64) Rand {
	return rand.New(rand.NewSource(seed))
}
