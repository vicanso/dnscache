.PHONY: default test test-cover dev

defalt: dev

lint:
	golangci-lint run

# for test
test:
	go test -race -cover ./...

test-cover:
	go test -race -coverprofile=test.out ./... && go tool cover --html=test.out

bench:
	go test -bench=".*" ./
