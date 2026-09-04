package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"
)

// import (
// 	"fmt"
// 	"testing"
// )

// func BenchmarkManyShortCommits(b *testing.B) {
// 	h1 := NewTestHelper(b)
// 	h1.Run("init")

// 	for n := 0; n < b.N; n++ {
// 		// Create 10 short commits
// 		for i := 0; i <= 10; i++ {
// 			h1.WriteFile(fmt.Sprintf("%d_%d.txt", n, i), "text")
// 			h1.Run("commit", fmt.Sprintf("Create %d_%d.txt", n, i))
// 		}
// 	}
// }

func DISABLED_TestCommitPprof(t *testing.T) {
	h1 := NewTestHelperAt("test_data", t)
	f, _ := os.Create("cpu.prof")
	for p := 0; p < 10; p++ {
		for n := 0; n < 200; n++ {
			h1.WriteFile(
				filepath.Join(fmt.Sprintf("d%d", p),
					fmt.Sprintf("file%d", n))+".txt",
				"aaaa")
		}
	}
	h1.Run("init")
	start := time.Now()
	pprof.StartCPUProfile(f)
	h1.Run("commit", "my commit")
	pprof.StopCPUProfile()
	elapsed := time.Since(start)
	fmt.Printf("commit took %s\n", elapsed)

	{
		f, _ := os.Create("heap.prof")
		pprof.WriteHeapProfile(f)
		f.Close()
	}
	{
		f, _ := os.Create("alloc.prof")
		pprof.Lookup("allocs").WriteTo(f, 0)
		f.Close()
	}
	// Inspect results with:
	// `go tool pprof cpu.prof`
	// `go tool pprof alloc.prof`
	// `go tool pprof heap.prof`
}

func DISABLED_TestStatusPprof(t *testing.T) {
	h1 := NewTestHelperAt("test_data", t)
	f, _ := os.Create("cpu.prof")
	for p := 0; p < 100; p++ {
		for n := 0; n < 200; n++ {
			h1.WriteFile(
				filepath.Join(fmt.Sprintf("d%d", p),
					fmt.Sprintf("file%d", n))+".txt",
				"aaaa")
		}
	}
	h1.Run("init")
	h1.Run("commit", "my commit")
	start := time.Now()
	pprof.StartCPUProfile(f)
	h1.Run("status")
	pprof.StopCPUProfile()
	elapsed := time.Since(start)
	fmt.Printf("status took %s\n", elapsed)

	{
		f, _ := os.Create("heap.prof")
		pprof.WriteHeapProfile(f)
		f.Close()
	}
	{
		f, _ := os.Create("alloc.prof")
		pprof.Lookup("allocs").WriteTo(f, 0)
		f.Close()
	}
	// Inspect results with:
	// `go tool pprof cpu.prof`
	// `go tool pprof alloc.prof`
	// `go tool pprof heap.prof`
}

func BenchmarkStatus(b *testing.B) {
	h := NewTestHelperNoCleanup(b)
	// h.Run("init")
	// for _, f0 := range []string{"a", "b", "c", "d", "e", "f", "g"} {
	// 	h.WriteFile(filepath.Join(f0)+".txt", "aaa")
	// 	for _, f1 := range []string{"a", "b", "c"} {
	// 		h.WriteFile(filepath.Join(f0, f1)+".txt", "aaa")
	// 		for _, f2 := range []string{"a", "b", "c"} {
	// 			h.WriteFile(filepath.Join(f0, f1, f2)+".txt", "aaa")
	// 		}
	// 	}
	// }
	// h.Run("commit", "first")
	// h.WriteFile("a.txt", "B")

	for n := 0; n < b.N; n++ {
		h.Run("status")
	}
}
