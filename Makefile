.PHONY: test test-race bench fuzz vet build clean testdata

build:
	@mkdir -p bin
	go build -o bin/ozainfo    ./cmd/ozainfo/
	go build -o bin/ozacat     ./cmd/ozacat/
	go build -o bin/ozaserve   ./cmd/ozaserve/
	go build -o bin/ozasearch  ./cmd/ozasearch/
	go build -o bin/ozaverify  ./cmd/ozaverify/
	go build -o bin/ozamcp     ./cmd/ozamcp/
	go build -o bin/zim2oza    ./cmd/zim2oza/

test:
	go test ./... -count=1

test-race:
	go test -race ./... -count=1

bench:
	go test -bench=. -benchmem ./oza/ ./ozawrite/

fuzz:
	go test -fuzz=FuzzParseHeader        -fuzztime=30s ./oza/
	go test -fuzz=FuzzParseVarEntryRecord -fuzztime=30s ./oza/
	go test -fuzz=FuzzParseSectionDesc   -fuzztime=30s ./oza/
	go test -fuzz=FuzzParseMIMETable     -fuzztime=30s ./oza/
	go test -fuzz=FuzzParseMetadata      -fuzztime=30s ./oza/
	go test -fuzz=FuzzParseTrigramIndex  -fuzztime=30s ./oza/
	go test -fuzz=FuzzDecodePostingList  -fuzztime=30s ./oza/
	go test -fuzz=FuzzParseIndex         -fuzztime=30s ./oza/

vet:
	go vet ./...

testdata: testdata/small.zim

testdata/small.zim:
	@mkdir -p testdata
	curl -sL "https://github.com/openzim/zim-testing-suite/raw/main/data/nons/small.zim" -o $@

bench-convert: build testdata
	@echo "Converting small.zim to OZA..."
	time ./bin/zim2oza --verbose testdata/small.zim /tmp/bench_output.oza
	./bin/ozaverify --all /tmp/bench_output.oza
	./bin/ozainfo /tmp/bench_output.oza

ZIM ?= $(error ZIM is not set. Usage: make bench-convert-large ZIM=/path/to/file.zim)
OZA_OUT ?= /tmp/bench_large.oza

bench-convert-large: build
	@echo "Converting $(ZIM) to OZA..."
	time ./bin/zim2oza --verbose "$(ZIM)" "$(OZA_OUT)"
	./bin/ozaverify --all --quiet "$(OZA_OUT)"
	./bin/ozainfo "$(OZA_OUT)"

clean:
	rm -rf bin/
