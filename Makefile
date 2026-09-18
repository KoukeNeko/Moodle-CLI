.PHONY: build install test test-race arch lint verify moodle-up moodle-down moodle-purge moodle-status help

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(BUILD_DATE)

# 測試用 Moodle 版本：v45 | v51 | v52
V ?= v52

help:
	@echo "開發："
	@echo "  make build                 編譯到 bin/moodle"
	@echo "  make test                  單元與契約測試"
	@echo "  make test-race             同上，開 race detector"
	@echo "  make arch                  只跑 import 邊界檢查"
	@echo "  make lint                  go vet"
	@echo "  make verify                test + test-race + lint + build"
	@echo
	@echo "測試環境（見 test/e2e/README.md）："
	@echo "  make moodle-up     V=v52   起容器 → 等就緒 → 佈建"
	@echo "  make moodle-down   V=v52   停掉，保留資料"
	@echo "  make moodle-purge  V=v52   停掉並刪除資料"
	@echo "  make moodle-status         列出目前的測試站"

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/moodle ./cmd/moodle

install: build
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 0755 bin/moodle "$(DESTDIR)$(BINDIR)/moodle"

# -count=1 是必要的，不是偏好：tests/arch 在執行期才讀取其他 package，
# Go 的測試快取看不到那個相依，會在邊界被打破後仍然重播舊的通過結果。
test:
	go test -count=1 ./...

test-race:
	go test -count=1 -race ./...

arch:
	go test -count=1 ./tests/arch/

lint:
	go vet ./...

verify: test test-race lint build

moodle-up:
	./scripts/moodle-env.sh up $(V)

moodle-down:
	./scripts/moodle-env.sh down $(V)

moodle-purge:
	./scripts/moodle-env.sh down $(V) --purge

moodle-status:
	./scripts/moodle-env.sh status
