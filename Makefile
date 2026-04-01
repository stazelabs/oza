.PHONY: test test-race cover cover-html lint lint-fix bench fuzz vet build clean testdata testdata-bench snapshot

build:
	@mkdir -p bin
	cd cmd && go build -o ../bin/ozainfo    ./ozainfo/
	cd cmd && go build -o ../bin/ozacat     ./ozacat/
	cd cmd && go build -o ../bin/ozaserve   ./ozaserve/
	cd cmd && go build -o ../bin/ozasearch  ./ozasearch/
	cd cmd && go build -o ../bin/ozaverify  ./ozaverify/
	cd cmd && go build -o ../bin/ozamcp     ./ozamcp/
	cd cmd && go build -o ../bin/zim2oza    ./zim2oza/

test:
	go test ./... -count=1
	cd cmd && go test ./... -count=1

test-race:
	go test -race ./... -count=1
	cd cmd && go test -race ./... -count=1

cover:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

cover-html: cover
	go tool cover -html=coverage.out

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

lint:
	golangci-lint run ./...
	cd cmd && golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...
	cd cmd && golangci-lint run --fix ./...

vet:
	go vet ./...
	cd cmd && go vet ./...

testdata: testdata/.stamp-tier1

testdata/.stamp-tier1:
	@mkdir -p testdata
	bash testdata/fetch.sh
	@touch $@

testdata-bench: testdata
	@mkdir -p testdata/bench
	bash testdata/fetch-bench.sh

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

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin/
