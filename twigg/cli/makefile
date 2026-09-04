benchmark-short-commits:
	go test -bench=BenchmarkManyShortCommits -run=^a -cpuprofile cpu.prof -memprofile=mem.prof
benchmark-large-commit:
	go test -bench=BenchmarkLargeCommit -run=^a -cpuprofile cpu.prof -memprofile=mem.prof
benchmark-status:
	go test -bench=BenchmarkStatus -run=^a -cpuprofile cpu.prof -memprofile=mem.prof

# Then type "web" or "pdf" to view the results
show-benchmark:
	go tool pprof mem.prof