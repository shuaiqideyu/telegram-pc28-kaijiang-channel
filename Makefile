.PHONY: build linux test

# 默认产物名 bobao
build:
	go build -o bobao .

linux:
	GOOS=linux GOARCH=amd64 go build -o bobao-linux .

test:
	go test ./...
