//go:build race

package en16931

// raceEnabled is true when the tests run with the race detector, which
// allocates on its own.
const raceEnabled = true
