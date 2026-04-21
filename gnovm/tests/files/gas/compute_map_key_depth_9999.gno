// MAXALLOC: 500000000
//
// Shows gas consumed when a 9 999-level [1]any{} chain is used as a map key.
//
// The total gas is dominated by building the chain (~856 000 gas/level at
// the current calibration), not by ComputeMapKey traversal:
//
//   ComputeMapKey contribution: 9 999 levels × OpCPUComputeMapKey(10) ≈ 100 000 gas
//   Build contribution:         9 999 levels × ~856 000 gas            ≈ 8 557 000 000 gas
//
// Extrapolating to the Go-stack crash depth (~1 900 000 levels):
//
//   ComputeMapKey gas at crash depth: 1 900 000 × 10 = 19 000 000
//   MaxGas on-chain:                                   3 000 000 000
//   Gas headroom:                              ×157 — crash happens long before OOG
//
// Conclusion: OpCPUComputeMapKey=10 is too cheap to stop the attack.
// The depth guard (maxComputeMapKeyDepth=10 000) is the correct defence.
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
// 8561226876
