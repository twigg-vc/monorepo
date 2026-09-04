fuzz:
	go test -fuzz=FuzzWriteAndRead -fuzztime=10s && \
	go test -fuzz=FuzzCopy -fuzztime=10s

test: fuzz
	go test ./...