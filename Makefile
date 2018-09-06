VERSION := $(shell git describe --tags --always --dirty="-dev")
LDFLAGS := -ldflags='-X "main.version=$(VERSION)"'
Q=@

GOTESTFLAGS = -race
ifndef Q
GOTESTFLAGS += -v
endif

.PHONY: deps
deps:
	$Qdep ensure

.PHONY: clean
clean:
	$Qrm -rf vendor/ && git checkout ./vendor && dep ensure

.PHONY: vet
vet:
	$Qgo vet ./...

.PHONY: fmtcheck
fmtchk:
	$Qexit $(shell goimports -l . | grep -v '^vendor' | wc -l)

.PHONY: fmtfix
fmtfix:
	$Qgoimports -w $(shell find . -iname '*.go' | grep -v vendor)

.PHONY: test
test: vet fmtcheck
	$Qgo test $(GOTESTFLAGS) -coverpkg="./..." -coverprofile=.coverprofile ./...
	$Qgo tool cover -func=.coverprofile
