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
// How to reproduce the crash
// ---
// 1. In gnovm/pkg/gnolang/values.go, raise (or remove) the depth guard:
//        const maxComputeMapKeyDepth = 2_000_000  // was 10_000
// 2. Run:
//        cd gnovm && go1.24.13 test ./pkg/gnolang/ \
//            -run "TestFiles$/gas/compute_map_key_crash_depth_known" \
//            -timeout=2h
//    (the test is known, so you must temporarily rename it or use a custom runner)
//
// ---
// Why gas alone cannot stop this attack
// ---
// Gas at this depth is dominated by ComputeMapKey traversal, not building:
//
//   The depth guard fires at 10 000 levels, so the build loop never finishes.
//   If the guard were absent and MaxGas were unenforced (as in filetests):
//
//   ComputeMapKey gas:  1 900 000 levels × OpCPUComputeMapKey(10) = 19 000 000
//   MaxGas on-chain:                                                3 000 000 000
//   Gas headroom:                                                          ×157
//
//   The stack crashes at ~1 900 000 levels — long before OOG.
//
// Note: with the merged gas model (PR #5291), building a 1 900 000-deep chain
// costs ~1.6 trillion gas (≈1 900 000 × 856 000 gas/level), which is already
// ~530× over MaxGas. So in production the build phase is independently
// gas-gated. But ComputeMapKey itself (10 gas/level) is not — which is why
// the depth guard is the correct and necessary defence.
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
