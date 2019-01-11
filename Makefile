GO=env GO111MODULE=on go
GONOMOD=env GO111MODULE=off go
IPFSCONTAINERS=`docker ps -a -q --filter="name=ipfs-*"`
COMPOSECOMMAND=env ADDR_NODE1=1 ADDR_NODE2=2 docker-compose -f testenv/docker-compose.yml
VERSION=`git describe --always --tags`

all: deps check build

.PHONY: build
build:
	go build -ldflags "-X main.Version=$(VERSION)"

.PHONY: install
install: deps
	go install -ldflags "-X main.Version=$(VERSION)"

.PHONY: config
config: build
	./ipfs-orchestrator -config ./config.example.json init

# Install dependencies
.PHONY: deps
deps:
	$(GO) mod vendor
	$(GO) get github.com/UnnoTed/fileb0x
	$(GO) get github.com/maxbrunsfeld/counterfeiter
	$(GO) mod tidy

# Run simple checks
.PHONY: check
check:
	go vet ./...
	go test -run xxxx ./...

# Execute tests
.PHONY: test
test:
	go test -race -cover ./...

.PHONY: testenv
testenv:
	$(COMPOSECOMMAND) up -d postgres

# Clean up containers and things
.PHONY: clean
clean:
	$(COMPOSECOMMAND) down
	docker stop $(IPFSCONTAINERS) || true
	docker rm $(IPFSCONTAINERS) || true
	rm -f ./ipfs-orchestrator
	find . -name tmp -type d -exec rm -f -r {} +

# Gen runs all code generators
.PHONY: gen
gen:
	fileb0x b0x.yml
	counterfeiter -o ./ipfs/mock/ipfs.mock.go \
		./ipfs/ipfs.go NodeClient

.PHONY: release
release:
	bash .scripts/release.sh

#####################
# DEVELOPMENT UTILS #
#####################

NETWORK=test_network

.PHONY: new-network
new-network: build
	./ipfs-orchestrator -dev dev -config config.example.json db $(NETWORK)

.PHONY: start-network
start-network: build
	./ipfs-orchestrator -dev ctl StartNetwork Network=$(NETWORK)
