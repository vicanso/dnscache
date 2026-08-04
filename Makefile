.PHONY: default test test-cover lint bench hooks

default: test

lint:
	golangci-lint run

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -coverprofile=test.out ./... && go tool cover --html=test.out

bench:
	go test -bench=BenchmarkLookupHost -benchmem -count=1 -run=^$$ ./

hooks:
	cp hooks/* .git/hooks/
