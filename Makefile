.PHONY: moodle-up moodle-down moodle-purge moodle-status help

# 測試用 Moodle 版本：v45 | v51 | v52
V ?= v52

help:
	@echo "測試環境（見 test/e2e/README.md）："
	@echo "  make moodle-up     V=v52   起容器 → 等就緒 → 佈建"
	@echo "  make moodle-down   V=v52   停掉，保留資料"
	@echo "  make moodle-purge  V=v52   停掉並刪除資料"
	@echo "  make moodle-status         列出目前的測試站"
	@echo
	@echo "Go 相關目標（build / test / test-race / lint / test-integration）"
	@echo "在 Phase 1 建立 go.mod 後加入"

moodle-up:
	./scripts/moodle-env.sh up $(V)

moodle-down:
	./scripts/moodle-env.sh down $(V)

moodle-purge:
	./scripts/moodle-env.sh down $(V) --purge

moodle-status:
	./scripts/moodle-env.sh status
