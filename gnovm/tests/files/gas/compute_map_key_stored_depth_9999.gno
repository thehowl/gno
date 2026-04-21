// MAXALLOC: 500000000
//
// Gas consumed in main() when a pre-built 9 999-iteration [1]any chain
// (stored in a package var during init()) is used as a map key.
//
// init() builds the chain (~8.56 B gas, infeasible in one on-chain tx) —
// excluded from // Gas: by the init/main reset — so the measurement
// captures only the map-key operation in main().
//
// Result: ~1.54 M gas, well within MaxGas (3 B).
//
// This is the last depth that succeeds; 10 000 iterations triggers the
// depth guard (see compute_map_key_stored_depth_10000.gno).
//
// Contrast with compute_map_key_depth_9999.gno: when the chain is built
// in main() the Gas: is 8 561 226 097 (dominated by the build).
// Here the build is free (init) and the Gas: is 5 560× cheaper.
//
// On-chain attack scenario:
//   A realm can accumulate depth across multiple transactions via
//   func Extend() { stored = [1]any{stored} }   (O(1) gas per call).
//   After 3 transactions × ~3 500 levels, depth exceeds 10 000.
//   Without the depth guard, a single ComputeMapKey call would then
//   eventually crash the node at ~1.9 M levels (~543 transactions away).
//   The depth guard fires safely at 10 000 — well before the crash.
package main

var chain any

func init() {
	var a any = [1]int{0}
	for i := 0; i < 9999; i++ {
		a = [1]any{a}
	}
	chain = a
}

func main() {
	m := make(map[any]int)
	m[chain] = 1
	println("ok")
}

// Output:
// ok

// Gas:
// 1539164
