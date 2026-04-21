// MAXALLOC: 500000000
//
// Gas consumed in main() when a pre-built 10 000-iteration [1]any chain
// (stored in a package var during init()) is used as a map key.
//
// init() builds the chain (~8.56 B gas) — excluded from // Gas: —
// so the measurement captures only the map-key attempt in main().
//
// The depth guard fires: 10 000 iterations exceeds maxComputeMapKeyDepth.
// Gas before the panic (~335 K) is ~4.6× less than the success case
// (~1.54 M for 9 999 iterations) because the guard aborts the traversal
// before the full key string is built and the map write occurs.
package main

var chain any

func init() {
	var a any = [1]int{0}
	for i := 0; i < 10000; i++ {
		a = [1]any{a}
	}
	chain = a
}

func main() {
	m := make(map[any]int)
	m[chain] = 1
}

// Error:
// map key nesting depth limit exceeded

// Gas:
// 335602
