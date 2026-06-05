LD_FLAGS ="-s -w"
BUILD_DIR =./build

# Portable build: compile inside an old-glibc container so the resulting
# binary/plugins run on older targets (Debian 10 / RHEL 8, glibc 2.28).
# Building on a newer host (e.g. glibc 2.43) stamps newer GLIBC_* symbol
# requirements that older servers can't satisfy.
DOCKER_IMAGE =golang:1.20-buster
DOCKER_RUN =docker run --rm -v "$(CURDIR)":/src -w /src -u "$(shell id -u):$(shell id -g)" -e HOME=/tmp -e GOCACHE=/tmp/.gocache -e GOPATH=/tmp/go $(DOCKER_IMAGE)

clean:
	rm -rf $(BUILD_DIR)
	rm -f raidstat.tar.gz
build:
	mkdir -p $(BUILD_DIR)
	go build -ldflags=$(LD_FLAGS) -buildmode=plugin -o $(BUILD_DIR)/adaptec.so plugins/adaptec/main.go
	go build -ldflags=$(LD_FLAGS) -buildmode=plugin -o $(BUILD_DIR)/hp.so plugins/hp/main.go
	go build -ldflags=$(LD_FLAGS) -buildmode=plugin -o $(BUILD_DIR)/marvell.so plugins/marvell/main.go
	go build -ldflags=$(LD_FLAGS) -buildmode=plugin -o $(BUILD_DIR)/megacli.so plugins/megacli/main.go
	go build -ldflags=$(LD_FLAGS) -buildmode=plugin -o $(BUILD_DIR)/sas2ircu.so plugins/sas2ircu/main.go
	go build -ldflags=$(LD_FLAGS) -buildmode=plugin -o $(BUILD_DIR)/mdstat.so plugins/mdstat/main.go
	go build -ldflags=$(LD_FLAGS) -o $(BUILD_DIR)/raidstat main.go
	install -m 644 config.json $(BUILD_DIR)/config.json
# Same as `build` but runs in the old-glibc container for broad compatibility.
build-portable:
	rm -rf $(BUILD_DIR)
	$(DOCKER_RUN) make build
install: $(BUILD_DIR)/raidstat
	install -d /opt/raidstat
	install -m 644 $(BUILD_DIR)/adaptec.so /opt/raidstat/adaptec.so
	install -m 644 $(BUILD_DIR)/hp.so /opt/raidstat/hp.so
	install -m 644 $(BUILD_DIR)/marvell.so /opt/raidstat/marvell.so
	install -m 644 $(BUILD_DIR)/megacli.so /opt/raidstat/megacli.so
	install -m 644 $(BUILD_DIR)/sas2ircu.so /opt/raidstat/sas2ircu.so
	install -m 644 $(BUILD_DIR)/mdstat.so /opt/raidstat/mdstat.so
	install -m 755 $(BUILD_DIR)/raidstat /opt/raidstat/raidstat
	install -m 644 config.json /opt/raidstat/config.json
tar: $(BUILD_DIR)/raidstat
	tar cfz raidstat.tar.gz build --transform 's/build/raidstat/'

# install/tar consume an existing build; they do NOT rebuild, so a portable
# build is not clobbered by an accidental host rebuild. Run `make build` or
# `make build-portable` first.
$(BUILD_DIR)/raidstat:
	@echo "No build found in $(BUILD_DIR). Run 'make build' or 'make build-portable' first." >&2
	@exit 1

.PHONY: clean build build-portable install tar test
.DEFAULT_GOAL = build
