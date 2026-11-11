# Common tasks. Run "make" with no target to format, vet, test, and build.
BINARY := bootmedian
# Strip debug symbols from release builds to make them smaller.
RELEASE_FLAGS := -trimpath -ldflags "-s -w"

.PHONY: all fmt vet test race cover bench build dist data run study profile compare compare-table clean

all: fmt vet test build

fmt:            ## reformat code to the standard Go style
	gofmt -s -w .

vet:            ## catch suspicious code the compiler allows
	go vet ./...

test:           ## run every unit test
	go test -v ./...

race:           ## run the tests with the data race detector (checks the parallel code)
	go test -race ./...

cover:          ## test coverage report, opened in the browser
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

bench:          ## speed and memory benchmarks of the bootstrap package
	go test -run '^$$' -bench . -benchmem ./bootstrap

build:          ## executable for this computer
	go build -o $(BINARY) .

dist:           ## executables for Mac, Windows, and Linux cloud servers (Intel and AWS Graviton)
	GOOS=darwin  GOARCH=arm64 go build $(RELEASE_FLAGS) -o dist/$(BINARY)-mac-arm64 .
	GOOS=darwin  GOARCH=amd64 go build $(RELEASE_FLAGS) -o dist/$(BINARY)-mac-intel .
	GOOS=windows GOARCH=amd64 go build $(RELEASE_FLAGS) -o dist/$(BINARY).exe .
	GOOS=linux   GOARCH=amd64 go build $(RELEASE_FLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build $(RELEASE_FLAGS) -o dist/$(BINARY)-linux-arm64 .

data:           ## regenerate the datasets in data/ (needs R)
	Rscript R/generate-data.R

run: build      ## bootstrap the median of one dataset
	./$(BINARY) -file data/timing/normal-n10000.csv -B 1000

study: build    ## rerun the R Monte Carlo study in Go
	./$(BINARY) -study data/study -B 100

profile: build  ## CPU and memory profiles of a large run, then the top CPU users
	./$(BINARY) -file data/timing/normal-n100000.csv -B 1000 -log profile.log \
		-cpuprofile cpu.prof -memprofile mem.prof
	go tool pprof -top -nodecount=10 $(BINARY) cpu.prof

compare: build  ## time every R and Go version on the same data (takes several minutes; needs R)
	go run ./cmd/compare

compare-table:  ## rebuild results/comparison.md from saved results (no rerun)
	go run ./cmd/compare -table-only

clean:
	rm -rf $(BINARY) $(BINARY).exe *.prof *.log coverage.out
