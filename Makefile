
run:
	go run ./*.go -config config.yaml

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o mb_forwarder ./*.go

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mb_forwarder ./*.go

build-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o mb_forwarder ./*.go

build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o mb_forwarder ./*.go

build-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o mb_forwarder ./*.go

clean:
	rm -f mb_forwarder

.PHONY: run build clean