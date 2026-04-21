// MAXALLOC: 500000000000
//
// Demonstrates the fatal Go-stack overflow that occurs when ComputeMapKey
// recurses ~1 900 000 levels deep — the point at which the Go runtime runs
// out of goroutine stack and kills the process.
//
// THIS TEST IS INTENTIONALLY SKIPPED (_known.gno suffix).
// Running it without the depth guard crashes the node process.
//
// ---
// Why this is reachable despite per-transaction gas limits
// ---
// MaxGas on-chain is 3 000 000 000.  Building 1 level of [1]any nesting
// costs ~856 000 gas, so a single transaction can add at most ~3 500 levels.
//
// A realm can accumulate depth across multiple transactions:
//
//   func Extend() { stored = [1]any{stored} }
//
// Each call wraps only the stored top-level reference — O(1) gas regardless
// of current depth.  After N transactions, the chain is N × 3 500 levels
// deep.  At N ≈ 543 the depth reaches 1 900 000 and a single
// ComputeMapKey call crashes the node.
//
// ---
// Why gas alone cannot stop this attack
// ---
// ComputeMapKey charges 10 gas per level (OpCPUComputeMapKey=10):
//
//   At 1 900 000 levels:  1 900 000 × 10 = 19 000 000 gas
//   MaxGas on-chain:                        3 000 000 000 gas
//   Gas headroom:                                    ×157
//
// The ComputeMapKey call is never OOG before the stack crashes.
//
// ---
// The depth guard (maxComputeMapKeyDepth = 10 000) is the correct defence
// ---
// After just 3 transactions the chain exceeds 10 000 levels.  The guard
// then panics safely with "map key nesting depth limit exceeded" on the
// first ComputeMapKey call — long before the crash depth.
//
// ---
// How to reproduce the crash
// ---
// 1. In gnovm/pkg/gnolang/values.go, raise (or remove) the depth guard:
//        const maxComputeMapKeyDepth = 2_000_000  // was 10_000
// 2. Run:
//        cd gnovm && go1.24.13 test ./pkg/gnolang/ \
//            -run "TestFiles$/gas/compute_map_key_crash_depth_known" \
//            -timeout=2h
//    (the test is _known, so temporarily rename or patch files_test.go)
//
// See also: compute_map_key_depth_9999.gno (empirical gas measurement at
// depth 9 999, one level below the guard).
package main

func main() {
	var a any = [1]int{0}
	for i := 0; i < 1_900_000; i++ {
		a = [1]any{a}
	}
	m := make(map[any]int)
	m[a] = 1
	println("ok")
}

// Output:
// ok

// Gas:
// 0
