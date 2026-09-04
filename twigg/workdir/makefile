benchmark:
	go test -bench=BenchmarkTree -run=^a -cpuprofile cpu.prof

# Then type "web" or "pdf" to view the results
show-benchmark:
	go tool pprof cpu.prof