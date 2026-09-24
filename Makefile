.PHONY: build install test test-race arch lint verify docs-check report-data-test moodle-up moodle-down moodle-purge moodle-status moodle-decade moodle-roles moodle-role-preflight moodle-service-matrix moodle-read-matrix moodle-matrix moodle-scale report-site help

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
	@echo "  make lint                  gofmt 檢查 + go vet"
	@echo "  make verify                test + test-race + lint + build"
	@echo "  make docs-check            驗證自動產生的 wiki 沒有漂移"
	@echo "  make report-data-test      驗證 dashboard 證據契約與 no-skip 規則"
	@echo
	@echo "測試環境（見 test/e2e/README.md）："
	@echo "  make moodle-up     V=v52   起容器 → 等就緒 → 佈建"
	@echo "  make moodle-down   V=v52   停掉，保留資料"
	@echo "  make moodle-purge  V=v52   停掉並刪除資料"
	@echo "  make moodle-status         列出目前的測試站"
	@echo "  make moodle-decade V=v52   佈建並驗收十年情境"
	@echo "  make moodle-roles  V=v52   動態建立並匯出 runtime roles"
	@echo "  make moodle-role-preflight V=v52  逐一驗證角色憑證與 typed WS transport"
	@echo "  make moodle-service-matrix V=v52  實際執行每個角色未暴露 WS 函式的拒絕邊界"
	@echo "  make moodle-read-matrix V=v52  實際執行每個角色已暴露的安全唯讀函式"
	@echo "  make moodle-matrix V=v52   小型代表性命令與十年正確性矩陣"
	@echo "  make moodle-scale  V=v52   50k 學生 PostgreSQL 規模測試"
	@echo "  make report-site           產生可發布的測試 dashboard"

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
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "these files are not gofmt'd:" >&2; echo "$$unformatted" >&2; exit 1; \
	fi
	go vet ./...

verify: test test-race lint build docs-check report-data-test

docs-check: build
	./scripts/generate-wiki.py --binary ./bin/moodle --check

report-data-test:
	python3 ./scripts/test-report-data.py
	python3 ./scripts/test-read-matrix.py

moodle-up:
	./scripts/moodle-env.sh up $(V)

moodle-down:
	./scripts/moodle-env.sh down $(V)

moodle-purge:
	./scripts/moodle-env.sh down $(V) --purge

moodle-status:
	./scripts/moodle-env.sh status

moodle-decade: build
	./test/e2e/decade-run.sh $(V)

moodle-roles:
	./test/e2e/role-fixture.sh $(V)

moodle-role-preflight: build
	./test/e2e/role-preflight.sh $(V)

moodle-service-matrix: build
	./test/e2e/service-matrix.py $(V)

moodle-read-matrix: build
	python3 ./test/e2e/read-matrix.py $(V)

moodle-matrix: build
	./scripts/moodle-env.sh up $(V)
	./test/e2e/decade-ci.sh $(V)
	STD_PORT=$$(case "$(V)" in v45) echo 8451;; v51) echo 8511;; v52) echo 8521;; *) exit 2;; esac); \
	NOWS_PORT=$$(case "$(V)" in v45) echo 8452;; v51) echo 8512;; v52) echo 8522;; *) exit 2;; esac); \
	STD_PORT=$$STD_PORT NOWS_PORT=$$NOWS_PORT ./test/e2e/full-run.sh --nows

moodle-scale:
	./test/scale/run.sh $(V)

report-site:
	./scripts/report-site.sh
