.PHONY: check test coverage vulncheck trivy secrets-scan verify build clean

check:
	@test -z "$$(gofmt -s -l .)" || (echo "Unformatted files found. Run 'gofmt -s -w .' to fix them." && false)
	golangci-lint run ./...
	go build ./...

test:
	go test -v -race ./...

coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

vulncheck:
	govulncheck ./...

trivy:
	trivy fs --severity CRITICAL,HIGH .

secrets-scan:
	gitleaks detect --source . --no-git --no-color --verbose

verify: check test vulncheck trivy secrets-scan

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.AppVersion=$(VERSION)" -o audiobook-tools ./cmd/audiobook-tools

clean:
	rm -rf coverage *.out .gocache
