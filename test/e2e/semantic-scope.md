# 語意範圍：空回應能證明什麼

> 狀態：進行中（2026-09-19）。起因是同一類缺陷在四個命令上各犯一次。

## 為什麼需要這一份

現有的 `scripts/audit-optional-fields.sh` 查的是「欄位在不在」——`VALUE_OPTIONAL`
宣告的欄位缺席時，不能讀成 false 或 0。那份稽核查不到另一類缺陷：

> **這支函式在回傳 `[]` 之前，搜尋的宇宙是什麼？**

形式化：命令宣稱回答謂詞 `C(x)`，端點只回滿足 `E(x)` 的東西。空回應要能支撐
「沒有任何 C」這句話，需要對**所有合法 Moodle 狀態**都成立：

```
C(x) ⇒ E(x)
```

只要存在一個合法狀態使 `C(x) ∧ ¬E(x)`，`端點 == []` 就證明不了 `¬∃x C(x)`。
這一條方程式涵蓋本專案至今找到的全部同類缺陷。

**不能從 `externallib` 宣告機械推導。** 那些宣告描述的是形狀不是完整性，
`db/services.php` 的能力後設資料也只是粗略的准入控制，兩者都看不到函式進入之後
套用的縮減。掃描 Moodle 原始碼可以找出約一半的「縮減候選」
（`enrol_get_*courses`、`has_capability`、`uservisible`、`groups_*`、
`submissions_open`、跳過紀錄的 `continue`、分頁、偏好設定、累積的 warnings），
但那是候選不是證明。

## 怎麼用

1. 每加一支依賴的 WS 呼叫，就在下表補一列，**最後一欄是產物**。
2. 想驗證某一列，用 `mutate.sh`：保留物件本身，只動一個維度，
   看命令會不會把「端點不回傳了」講成「這東西不存在」。
3. 受測的性質不是「端點應該還是要回傳」——通常它正確地不該回傳。

## 表

| 命令 | 主要呼叫 | 種子宇宙 | 已知縮減 | 空回應**不能**證明 |
|---|---|---|---|---|
| `course list` | `core_enrol_get_users_courses` | 這個帳號的選課 | `onlyactive` 寫死 true；隱藏課程；已結束；沒有選課但讀得到的課程不在裡面 | 這個帳號沒有選過課。**實測**：停權一筆選課，清單就空了 |
| `assignment list` | `mod_assign_get_assignments` | 不指定課程時＝選課的課程 | 選課；課程可見性；投影會依繳交狀態省略欄位 | 沒有作業。**實測**：類別層的 manager 讀得到一門有兩份作業的課，清單是空的 |
| `forum list` | `mod_forum_get_forums_by_courses` | 不指定課程時＝選課的課程 | 選課；活動可見性；`mod/forum:viewdiscussion`；被過濾的課程只進 warnings | 這個帳號看不到論壇。**實測**：同一個 manager 指定課程就讀到了 |
| `forum discussions` | `mod_forum_get_forum_discussions` | 該論壇 | `can_view_discussion()`；Q&A 論壇在自己發文前看不到別人的 | 這個論壇沒有討論串。**未實測** |
| `grade list` | `gradereport_user_get_grade_items` | 該課程該帳號 | 項目 `hidden`／hidden-until；user report 的可見度設定；含隱藏項目的總分也會被藏 | 沒有成績。**實測**：整門課項目設隱藏後報表全空，而 92、78.5 與總分 170.50 都還在 |
| `grade overview` | `gradereport_overview_get_course_grades` | 這個帳號被評分的課程 | 只含被評分者（教職員不在內）；跳過 `showgrades=0` 的課程；站台隱藏的課程 | 沒有課程總分。**實測**：教 31 門課的教師、封存課裡有 135 分的學生、關掉 showgrades 後有 20 筆總分的學生，三種都是空的 |
| `calendar upcoming` | `core_calendar_get_calendar_monthly_view`＋`core_calendar_get_action_events_by_timesort` | 月曆為基底，動作清單左接 | 月檢視套用行事曆的可見度；動作清單只回有動作回呼的事件，但預設**不**排除停權選課的課程 | 沒有到期的事情。**實測**：改用動作清單當唯一來源時，學生看到四分之一、教師看到零 |
| `file download` | `pluginfile.php` | 該檔案 | 授權；某些 file area 用同一支 `send_file_not_found()` 回應兩種情況 | — 這條**實測不成立**：webservice/pluginfile 以 `requireloginerror`（HTTP 200 JSON）與 `filenotfound`（HTTP 404）區分兩者，CLI 已分別對應 exit 5 與 6 |
| `site inspect` / `doctor` | `core_webservice_get_site_info` | **token 所屬的 external service** | 服務沒開的函式不在清單裡 | 這個站台沒有這支函式。**實測**：從 `moodle_mobile_app` 移除一支站台仍然裝著的函式 |
| `auth methods` | `tool_mobile_get_public_config` | 不需認證的公開設定 | 文件明說只回公開設定 | 對應的私有設定不存在。**未實測** |

## 還沒查的

- `forum discussions` 的 Q&A 情境（研究列為中風險）。
- `assignment status/show` 的投影：`lastattempt` 之類的子結構綁在能力上。
- `auth methods` 的公開設定缺席。
