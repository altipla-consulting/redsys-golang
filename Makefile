
FILES = $(shell find . -type f -name '*.go')

lint:
	linter ./...
	go vet ./...
	go install ./...

test:
	go test -race -v ./...

gofmt:
	@gofmt -s -w $(FILES)
	@gofmt -r '&α{} -> new(α)' -w $(FILES)
	@impsort . -p github.com/altipla-consulting/redsys-golang
