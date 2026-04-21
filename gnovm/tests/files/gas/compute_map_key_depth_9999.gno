// MAXALLOC: 500000000
//
// Gas consumed when a 9 999-level [1]any{} chain is used as a map key.
// (Note: // Gas: measures main() only; package-init overhead is excluded.)
//
// The build cost dominates: ~856 000 gas/level at the current calibration.
// ComputeMapKey itself adds only 10 gas/level (OpCPUComputeMapKey).
//
//   Build (9 999 levels): ~856 000 gas/level × 9 999  ≈  8 557 000 000 gas
//   ComputeMapKey:         10 gas/level    × 9 999  ≈      100 000 gas
//
// MaxGas on-chain: 3 000 000 000
//
// Single-transaction build limit: 3 000 000 000 / 856 000 ≈ 3 500 levels.
//
// So 9 999 levels is impossible in a single transaction.  However, a realm
// can accumulate depth across multiple transactions: each call to
//
//   func Extend() { stored = [1]any{stored} }
//
// wraps only the top-level reference (O(1) gas), adding one level at
// ~856 000 gas.  Three transactions of MaxGas can each add ~3 500 levels,
// reaching depth 10 500 — past the 10 000-level depth guard.
//
// The depth guard (maxComputeMapKeyDepth=10 000) is therefore the correct
// defence: it panics safely on the 4th transaction's ComputeMapKey call,
// long before the Go-stack overflow at ~1 900 000 levels.
//
// Raising OpCPUComputeMapKey would not help: even at the guard boundary
// (10 000 levels × 10 gas = 100 000 gas) the gas contribution is negligible.
//
// See also: compute_map_key_crash_depth_known.gno
package main

func main() {
	var a any = [1]int{0}
	for i := 0; i < 9999; i++ {
		a = [1]any{a}
	}
	m := make(map[any]int)
	m[a] = 1
	println("ok")
}

// Output:
// ok

// Gas:
// 8561226097
