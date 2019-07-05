# You may need to update this to reflect your PYTHONPATH.
PKG_CONFIG_PATH=${CONDA_PREFIX}/lib/pkgconfig
LD_LIBRARY_PATH=${CONDA_PREFIX}/lib/python3.7:${CONDA_PREFIX}/lib
PYTHONPATH=${CONDA_PREFIX}/lib/python3.7/site-packages:${PWD}/__python__
GO_CMD=PKG_CONFIG_PATH=${PKG_CONFIG_PATH} LD_LIBRARY_PATH=${LD_LIBRARY_PATH} PYTHONPATH=${PYTHONPATH} go

GO_BUILD=$(GO_CMD) build
GO_CLEAN=$(GO_CMD) clean
GO_TEST?=$(GO_CMD) test
GO_GET=$(GO_CMD) get
GO_RUN=${GO_CMD} run

DIST_DIR=bin

GO_SOURCES := $(shell find . -path -prune -o -name '*.go' -not -name '*_test.go')
SOURCES_NO_VENDOR := $(shell find . -path ./vendor -prune -o -name "*.go" -not -name '*_test.go' -print)
CMD_TOOLS := $(shell find ./cmd -path -prune -o -name "*.go" -not -name '*_test.go')

.PHONY: default clean test build fmt bench run ci

#
# Our default target, clean up, do our install, test, and build locally.
#
default: clean build

# Clean up after our install and build processes. Should get us back to as
# clean as possible.
#
clean:
	# go clean -cache -testcache -modcache
	@for d in ./bin/*; do \
		if [ -f $$d ] ; then rm $$d ; fi \
	done
	rm -rf ./__python__/*.pyc

#
# Do what we need to do to run our tests.
#
test: clean $(GO_SOURCES)
	$(GO_TEST) -count=1 -v $(GO_TEST_ARGS) ./...

#
# Build/compile our application.
#
build:
	@for d in ./cmd/*; do \
		echo "Building ${DIST_DIR}/`basename $$d`"; \
		${GO_BUILD} -ldflags="-L ${LD_LIBRARY_PATH}" -o ${DIST_DIR}/`basename $$d` $$d; \
	done

#
# Format the sources.
#
fmt: $(SOURCES_NO_VENDOR)
	goimports -w $^

#
# Run the benchmarks for the tools.
#
bench: $(GO_SOURCES)
	$(GO_TEST) $(GO_TEST_ARGS) -bench=. -run=- ./...

#
# Most of this is setup with telling python c-api where the python modules are.
#
run: clean build
	${GO_RUN} ./bin/task_example

ci:
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
	echo "deb http://deb.debian.org/debian unstable main" >> /etc/apt/sources.list
	apt-get update && apt-get install -y python3.7-dev
	cp /usr/lib/x86_64-linux-gnu/pkgconfig/python-3.7.pc /usr/lib/x86_64-linux-gnu/pkgconfig/python3.pc
	PYTHONPATH=${PWD}/__python__ go test -cover ./...