BUILD_VERSION   ?= $(shell cat VERSION || echo "0.0.0")
BUILD_DATE      := $(shell date "+%Y%m%d")
COMMIT_SHA1     := $(shell git rev-parse --short HEAD || echo "abcd1234")
RELEASEV := ${BUILD_VERSION}-${BUILD_DATE}-${COMMIT_SHA1}
IMAGE           ?= ysicing/kubetls

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## fmt code
	# addlicense -f licenses/z-public-1.2.tpl -ignore web/** ./**
	gofmt -s -w .
	goimports -w .
	@echo gofmt -l
	@OUTPUT=`gofmt -l . 2>&1`; \
	if [ "$$OUTPUT" ]; then \
		echo "gofmt must be run on the following files:"; \
        echo "$$OUTPUT"; \
        exit 1; \
    fi

lint: ## lint code
	@echo golangci-lint run --skip-files \".*test.go\" -v ./...
	@OUTPUT=`command -v golangci-lint >/dev/null 2>&1 && golangci-lint run --skip-files ".*test.go"  -v ./... 2>&1`; \
	if [ "$$OUTPUT" ]; then \
		echo "go lint errors:"; \
		echo "$$OUTPUT"; \
	fi



default: fmt lint ## fmt code

run: ## 运行
	air

build: ## 构建二进制
	@echo "build bin ${RELEASEV}"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o dist/kubetls \
    	-ldflags "-X 'github.com/ysicing/kubetls/constants.Commit=${COMMIT_SHA1}' \
          -X 'github.com/ysicing/kubetls/constants.Date=${BUILD_DATE}' \
          -X 'github.com/ysicing/kubetls/constants.Release=${BUILD_VERSION}'" cmd/kubetls.go

dev-docker: ## 构建测试镜像
	docker build -t ${IMAGE}/kubetls:${BUILD_VERSION} .

dev-push: dev-docker
	docker push ${IMAGE}/kubetls:${BUILD_VERSION}

.PHONY : dev-push

.EXPORT_ALL_VARIABLES:

GO111MODULE = on
GOPROXY = https://goproxy.cn,direct
GOPRIVATE = gitlab.zcorp.cc
GOSUMDB = sum.golang.google.cn
