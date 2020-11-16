# vim: noexpandtab

export GO111MODULE = on
export CGO_ENABLED = 1
export CGO_CFLAGS = -g -O2 -Wno-return-local-addr

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

all:
	$(error Use make (dev|release|static_release) instead)

race:
	./tools/go_test.sh -race

dev: postfix_parser  mocks swag domain_mapping_list po2go recommendation_dev
	go build -tags="dev no_postgres no_mysql no_clickhouse no_mssql" -o "lightmeter" -ldflags "${BUILD_INFO_FLAGS}"

release: postfix_parser static_www domain_mapping_list po2go recommendation_release
	go build -tags="release no_postgres no_mysql no_clickhouse no_mssql" -o "lightmeter" -ldflags "${BUILD_INFO_FLAGS}"

windows_release: postfix_parser static_www domain_mapping_list po1go
	CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 go build -tags="release no_postgres no_mysql no_clickhouse no_mssql" -o "lightmeter.exe" -ldflags "${BUILD_INFO_FLAGS}"

static_release: postfix_parser static_www domain_mapping_list po2go
	go build -tags="release no_postgres no_mysql no_clickhouse no_mssql" -o "lightmeter" -ldflags \
		"${BUILD_INFO_FLAGS} -linkmode external -extldflags '-static' -s -w" -a -v

static_www:
	go generate -tags="release" gitlab.com/lightmeter/controlcenter/staticdata

domain_mapping_list: domainmapping/generated_list.go

domainmapping/generated_list.go: domainmapping/mapping.json
	go generate gitlab.com/lightmeter/controlcenter/domainmapping

recommendation_dev:
	go generate -tags="dev" gitlab.com/lightmeter/controlcenter/recommendation

recommendation_release:
	go generate -tags="release" gitlab.com/lightmeter/controlcenter/recommendation

mocks: postfix_parser dashboard_mock insights_mock

dashboard_mock:
	go generate -tags="dev" gitlab.com/lightmeter/controlcenter/dashboard

insights_mock:
	go generate -tags="dev" gitlab.com/lightmeter/controlcenter/insights/core

po2go:
	go generate -tags="dev" gitlab.com/lightmeter/controlcenter/po

code2po:
	sh ./tools/code2poutil.sh

go2po:
	sh ./tools/go2poutil.sh

swag:
	go generate -tags="dev" gitlab.com/lightmeter/controlcenter/api
	cp api/docs/swagger.json www/api.json

clean: clean_binaries clean_swag clean_staticdata clean_mocks clean_postfix_parser
	rm -f dependencies.svg

clean_binaries:
	rm -f lightmeter lightmeter.exe

clean_staticdata:
	rm -f staticdata/http_vfsdata.go

clean_swag:
	rm -f docs/docs.go docs/swagger.json docs/swagger.yaml www/api.json

clean_mocks:
	rm -f dashboard/mock/dashboard_mock.go

dependencies.svg: go.sum go.mod
	go mod graph | tools/gen_deps_graph.py | dot -Tsvg > dependencies.svg

make testlocal:
	./tools/go_test_local.sh

postfix_parser:
	go generate gitlab.com/lightmeter/controlcenter/pkg/postfix/logparser/rawparser

clean_postfix_parser:
	@rm -vf pkg/postfix/logparser/rawparser/*.gen.go
