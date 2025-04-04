generate-mock:
	mockery --all --config .mockery.yaml

test:
	go test -v -count=1 -race ./...
