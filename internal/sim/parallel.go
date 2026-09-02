package sim

import (
	"runtime"
	"sync"
)

// parallelFor splits CPU-heavy, independent work into stable contiguous
// ranges. Callers still own the simulation lock and must only write to the
// range assigned to each worker.
func parallelFor(count, threshold int, work func(start, end int)) {
	workers := min(runtime.GOMAXPROCS(0), count)
	if workers < 2 || count < threshold {
		work(0, count)
		return
	}
	chunk := (count + workers - 1) / workers
	var group sync.WaitGroup
	for start := 0; start < count; start += chunk {
		end := min(count, start+chunk)
		group.Add(1)
		go func(start, end int) {
			defer group.Done()
			work(start, end)
		}(start, end)
	}
	group.Wait()
}
