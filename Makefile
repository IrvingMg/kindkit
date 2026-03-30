.PHONY: test test-unit test-e2e test-examples

test: test-unit test-e2e

test-unit:
	go test -v ./...

test-e2e:
	go test -v -tags=e2e -timeout=5m ./test/e2e/...

test-examples:
	cd examples && go test -v -timeout=5m ./...
