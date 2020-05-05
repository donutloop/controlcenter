# vim: noexpandtab

export GO111MODULE = on
export CGO_ENABLED = 1

PACKAGE_ROOT = gitlab.com/lightmeter/controlcenter
PACKAGE_VERSION = ${PACKAGE_ROOT}/version
APP_VERSION = `cat VERSION.txt`

ifneq ($(wildcard .git),)
	GIT_COMMIT = `git rev-parse --short HEAD`
	GIT_BRANCH = `git describe --tags --exact-match 2>/dev/null || git symbolic-ref -q --short HEAD`
else
	GIT_COMMIT = ""
	GIT_BRANCH = ""
endif

BUILD_INFO_FLAGS = -X ${PACKAGE_VERSION}.Commit=${GIT_COMMIT} -X ${PACKAGE_VERSION}.TagOrBranch=${GIT_BRANCH} -X ${PACKAGE_VERSION}.Version=${APP_VERSION}

all: dev

dev: mocks swag
	go build -tags="dev" -o "lightmeter" -ldflags "${BUILD_INFO_FLAGS}"

release: static_www
	go build -tags="release" -o "lightmeter" -ldflags "${BUILD_INFO_FLAGS}"

windows_release: static_www
	CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 go build -tags="release" -o "lightmeter.exe" -ldflags "${BUILD_INFO_FLAGS}"

static_release: static_www 
	go build -tags="release" -o "lightmeter" -ldflags \
		"${BUILD_INFO_FLAGS} -linkmode external -extldflags '-static' -s -w" -a -v

static_www:
	go generate -tags="release" gitlab.com/lightmeter/controlcenter/staticdata

mocks:
	go generate -tags="dev" gitlab.com/lightmeter/controlcenter/dashboard

swag:
	go run github.com/swaggo/swag/cmd/swag init --generalInfo api/http.go
	cp docs/swagger.json www/api.json

clean: clean_binaries clean_swag clean_staticdata clean_mocks
	rm -f dependencies.svg

clean_binaries:
	rm -f lightmeter lightmeter.exe

clean_staticdata:
	rm -f staticdata/http_vfsdata.go

clean_swag:
	rm -f docs/docs.go docs/swagger.json docs/swagger.yaml www/api.json

clean_mocks:
	rm -f dashboard/mock/dashboard_mock.go

dependencies.svg:
	go mod graph | utils/gen_deps_graph.py | dot -Tsvg > dependencies.svg