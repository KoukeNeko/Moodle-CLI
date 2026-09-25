#!/usr/bin/env python3
"""Generate exhaustive bilingual command and core-function wiki references."""
from __future__ import annotations

import argparse
import json
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
WIKI = ROOT / "docs/wiki"

GROUP_ZH = {
    "api": "直接操作站台 Web Service 函式",
    "assignment": "查看、提交與管理作業",
    "auth": "登入 Moodle、管理憑證並檢查目前身分",
    "calendar": "查看與管理行事曆事件及待辦事項",
    "completion": "查看及更新課程／活動完成狀態",
    "course": "依 Moodle 角色允許的範圍讀取與管理課程",
    "enrolment": "查看選課方法並管理使用者選課",
    "file": "下載 Moodle 所保存的檔案",
    "forum": "讀取與管理討論區、主題及貼文",
    "grade": "讀取與管理成績及成績簿分類",
    "group": "管理課程群組及成員",
    "mcp": "以 Model Context Protocol 將 Moodle 提供給 agent",
    "participant": "搜尋與查看課程參與者",
    "site": "管理 CLI 已知的 Moodle 站台與學務設定",
    "workload": "依課程自訂欄位計算並驗證每學期學分",
    "ws": "檢視並呼叫具型別的 Moodle core Web Service registry",
}

ACTION_ZH = {
    "activity": "顯示活動完成狀態", "add": "新增", "academic": "管理學務欄位設定",
    "call": "呼叫函式", "category-create": "建立成績簿分類", "configure": "設定欄位對應與最低學分",
    "contents": "顯示課程章節、活動與檔案", "course": "顯示課程完成狀態",
    "create": "建立", "delete": "刪除", "describe": "顯示版本化 schema、effect 與需求",
    "discussions": "列出討論主題", "download": "下載", "extend": "設定繳交展延期限",
    "favourite": "加入最愛", "functions": "列出帳號可呼叫的函式", "grade": "評分",
    "handler-status": "檢查瀏覽器登入 handler", "import-browser": "匯入既有瀏覽器 session",
    "import-session": "從隱藏輸入匯入並驗證瀏覽器 session",
    "list": "列出項目", "lock": "鎖定", "login": "登入並安全保存憑證",
    "logout": "刪除本機保存的憑證", "mark": "手動標記完成", "member-add": "加入群組成員",
    "member-remove": "移除群組成員", "methods": "列出可用方法", "overview": "顯示總覽",
    "pin": "置頂", "read": "讀取", "register-handler": "安裝瀏覽器登入 handler",
    "remove": "移除", "reply": "回覆", "reveal-identities": "揭露匿名評分身分",
    "revert": "退回草稿", "search": "搜尋", "serve": "在 stdin/stdout 執行 MCP server",
    "show": "顯示詳細資料", "status": "顯示目前狀態", "submit": "提交作業並回讀狀態",
    "submissions": "列出作業繳交", "subscribe": "訂閱", "unfavourite": "移出最愛",
    "unlock": "解除鎖定", "unpin": "取消置頂", "unregister-handler": "移除瀏覽器登入 handler",
    "unsubscribe": "取消訂閱", "update": "更新", "use": "設為預設站台", "validate": "驗證最低學分",
}

SPECIAL_ZH = {
    "commands": "列出每個 CLI 命令、旗標、輸出 kind 與寫入屬性，供文件、script 與 agent 使用。",
    "doctor": "診斷站台、登入、backend 與 capability，並說明不可用的原因。",
    "resolve": "解析 Moodle URL 所指向的資源種類與識別碼。",
    "schema": "列出或輸出指定 JSON response kind 的 schema。",
    "version": "輸出 CLI 版本、commit 與建置時間。",
    "site add": "登記一個 Moodle 站台。",
    "site inspect": "檢查此帳號在站台可用的 capability 與 external functions。",
    "site remove": "移除站台設定及其本機憑證。",
    "ws list": "離線列出 Moodle 4.5、5.1、5.2 core external functions 聯集。",
    "workload show": "依學期顯示課程、學分、學制及適用最低學分。",
    "workload validate": "驗證各學期是否符合大學部或研究所最低學分。",
}

FLAG_ZH = {
    "account": "指定代為操作的帳號", "site": "指定 Moodle 站台", "json": "以版本化 JSON envelope 輸出",
    "pretty": "縮排 JSON 輸出", "read-only": "拒絕所有可能變更 Moodle 的操作",
    "backend": "限制可使用的 backend", "param": "加入一個 name=value 參數，可重複",
    "params-json": "以 JSON object 提供巢狀參數", "dry-run": "驗證並顯示操作，但不送出",
    "yes": "確認執行 Moodle 寫入", "allow-write": "允許呼叫已知寫入函式",
    "course": "限制或指定課程", "version": "指定 registry 的 Moodle 版本",
    "match": "只保留名稱包含此文字的項目", "writes": "只顯示可能變更資料的函式",
    "unreviewed": "只顯示尚未審查的函式", "require-minimum": "未達最低學分時回傳 validation exit code",
    "browser": "選擇 safari、firefox 或 chromium；未指定時搜尋現有 profile",
    "profile": "指定瀏覽器 profile 目錄或 Safari cookie 檔案",
    "cookie-name": "站台若改過 session cookie 名稱，可在此指定（預設 MoodleSession）",
    "list-profiles": "只列出此電腦找到的瀏覽器 profile",
    "store": "驗證後將 session 存入作業系統鑰匙圈",
    "stdin": "從標準輸入讀取單一 session cookie；未指定時使用隱藏輸入提示",
}

GLOBAL_FLAGS_EN = [
    ("--json", "Emit the versioned JSON contract on stdout."),
    ("--pretty", "Indent JSON output; meaningful with --json."),
    ("--read-only", "Hide and refuse every command that may mutate Moodle."),
    ("--backend auto|ws-only", "Allow automatic fallback routes, or restrict the run to Web Services."),
]
GLOBAL_FLAGS_ZH = [
    ("--json", "將版本化 JSON contract 寫到 stdout。"),
    ("--pretty", "縮排 JSON；與 --json 一起使用。"),
    ("--read-only", "隱藏並拒絕所有可能變更 Moodle 的命令。"),
    ("--backend auto|ws-only", "允許自動 fallback，或將本次執行限制為 Web Services。"),
]


def command_data(binary: pathlib.Path) -> list[dict]:
    result = subprocess.run([str(binary), "commands", "--json"], cwd=ROOT,
                            text=True, capture_output=True, check=True)
    return json.loads(result.stdout)["data"]


def usage(binary: pathlib.Path, path: str) -> str:
    result = subprocess.run([str(binary), *path.split(), "--help"], cwd=ROOT,
                            text=True, capture_output=True)
    match = re.search(r"(?m)^Usage:\s*\n\s{2}(.+)$", result.stdout + result.stderr)
    return match.group(1).strip() if match else f"moodle {path} [flags]"


def zh_description(row: dict) -> str:
    path = row["path"]
    if path in SPECIAL_ZH:
        return SPECIAL_ZH[path]
    parts = path.split()
    if len(parts) == 1:
        return GROUP_ZH.get(path, row["short"]) + "。"
    subject = GROUP_ZH.get(parts[0], parts[0])
    action = ACTION_ZH.get(parts[-1], row["short"])
    return f"{subject}：{action}。"


def write_command_reference(binary: pathlib.Path, rows: list[dict], zh: bool) -> str:
    title = "# 完整命令參考" if zh else "# Complete command reference"
    intro = (
        "本頁由 `moodle commands --json` 自動產生；每個公開命令都必須出現在此。"
        "位置參數以 synopsis 為準；網站、角色或 capability 不允許時，Moodle 會回傳明確錯誤。"
        if zh else
        "This page is generated from `moodle commands --json`; every public command must appear here. "
        "The synopsis is authoritative for positional arguments. Site, role, and capability restrictions are reported explicitly."
    )
    lines = [title, "", "[English](Command-Reference) · [繁體中文](Command-Reference-zh-TW)", "", intro, "",
             "## 全域旗標" if zh else "## Global flags", "", "| Flag | 說明 |" if zh else "| Flag | Meaning |", "| --- | --- |"]
    for flag, meaning in GLOBAL_FLAGS_ZH if zh else GLOBAL_FLAGS_EN:
        lines.append(f"| `{flag}` | {meaning} |")
    lines += ["", f"## {'命令' if zh else 'Commands'} ({len(rows)})", ""]
    for row in rows:
        lines += [f"### `moodle {row['path']}`", "", zh_description(row) if zh else row["short"], "",
                  f"- {'用法' if zh else 'Synopsis'}: `{usage(binary, row['path'])}`"]
        if row["kind"]:
            lines.append(f"- {'JSON 輸出 kind' if zh else 'JSON response kind'}: `{row['kind']}`")
        elif any(other["path"].startswith(row["path"] + " ") for other in rows):
            lines.append(f"- {'類型：命令群組，請選擇子命令' if zh else 'Type: command group; select a subcommand'}")
        lines.append(f"- {'資料效果' if zh else 'Data effect'}: **{'寫入' if zh and row['mutates'] else 'write' if row['mutates'] else '唯讀' if zh else 'read-only'}**")
        if row["flags"]:
            lines += ["", "| Flag | 說明 |" if zh else "| Flag | Meaning |", "| --- | --- |"]
            for flag in row["flags"]:
                syntax = f"-{'-' + flag['name']}"
                if flag["shorthand"]:
                    syntax = f"-{flag['shorthand']}, {syntax}"
                meaning = FLAG_ZH.get(flag["name"], flag["usage"]) if zh else flag["usage"]
                lines.append(f"| `{syntax}` | {meaning} |")
        lines += [""]
    return "\n".join(lines).rstrip() + "\n"


def function_rows() -> list[dict]:
    combined: dict[str, dict] = {}
    for version in ("v45", "v51", "v52"):
        doc = json.loads((ROOT / f"internal/wsregistry/data/{version}.json").read_text())
        for item in doc["functions"]:
            row = combined.setdefault(item["name"], {"name": item["name"], "component": item["component"],
                "effect": set(), "versions": [], "transports": set(), "deprecated": False})
            row["effect"].add(item["effect"])
            row["versions"].append(version)
            row["transports"].update(name.upper() for name, enabled in item.get("transports", {}).items() if enabled)
            row["deprecated"] = row["deprecated"] or bool(item.get("deprecated"))
    return [combined[name] for name in sorted(combined)]


def write_feature_coverage(rows: list[dict], zh: bool) -> str:
    title = "# 功能覆蓋" if zh else "# Feature coverage"
    intro = ("此表是 Moodle 4.5.12、5.1.7、5.2.3 core `external_functions` 的聯集。"
             "每個函式都可用 `moodle ws describe <function>` 查看版本化參數與回傳 schema，並以 `moodle ws call` 呼叫。"
             if zh else "This is the union of core `external_functions` from Moodle 4.5.12, 5.1.7, and 5.2.3. "
             "Use `moodle ws describe <function>` for versioned parameter/return schemas and `moodle ws call` to invoke it.")
    lines = [title, "", "[English](Feature-Coverage) · [繁體中文](Feature-Coverage-zh-TW)", "", intro, "",
             f"**{'聯集' if zh else 'Union'}: {len(rows)} {'個 core functions' if zh else 'core functions'}**", "",
             "Registry 收錄不等於所有角色都能執行；權限仍由 Moodle runtime 判定。" if zh else
             "Registry presence does not mean every role may execute the function; Moodle remains the runtime authority.", "",
             "| Function | Component | Effect | Versions | Transport | Deprecated |",
             "| --- | --- | --- | --- | --- | --- |"]
    for row in rows:
        effect = "/".join(sorted(row["effect"]))
        versions = ", ".join(row["versions"])
        transports = ", ".join(sorted(row["transports"])) or "—"
        deprecated = "是" if zh and row["deprecated"] else "yes" if row["deprecated"] else "否" if zh else "no"
        lines.append(f"| `{row['name']}` | `{row['component']}` | {effect} | {versions} | {transports} | {deprecated} |")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=pathlib.Path, default=ROOT / "bin/moodle")
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    rows = command_data(args.binary)
    generated = {
        WIKI / "Command-Reference.md": write_command_reference(args.binary, rows, False),
        WIKI / "Command-Reference-zh-TW.md": write_command_reference(args.binary, rows, True),
        WIKI / "Feature-Coverage.md": write_feature_coverage(function_rows(), False),
        WIKI / "Feature-Coverage-zh-TW.md": write_feature_coverage(function_rows(), True),
    }
    stale = [path for path, content in generated.items() if not path.exists() or path.read_text() != content]
    if args.check:
        if stale:
            print("generated wiki is stale: " + ", ".join(str(path.relative_to(ROOT)) for path in stale), file=sys.stderr)
            return 1
        return 0
    for path, content in generated.items():
        path.write_text(content)
        print(path.relative_to(ROOT))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
