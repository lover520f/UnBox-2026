# 点播加载反馈与跳过片头片尾实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让点播起播过程有明确加载反馈，并支持用户标记片头/片尾后自动跳过。

**Architecture:** 加载反馈由 App 依据播放器上报的 `playback` 事件驱动；跳过标记存 `store.kv`
（键 `vod.skip.<site>.<vodID>`），判定收敛为纯函数 `resolveSkipAction`，Web 与 mpv 分别用
`progress` 事件和 `playback:event` 位置事件驱动同一个判定。

**Tech Stack:** Go 1.26 / Wails v3 beta.9 / Vue 3 + TypeScript / Vitest / modernc SQLite KV。

**Spec:** `docs/superpowers/specs/2026-09-20-vod-skip-intro-design.md`

## Global Constraints

- 标记单位秒，`0` 表示未标记；读取失败或非法 JSON 一律按未标记处理，不得影响播放。
- 只对点播详情页生效；直播与本地媒体库不受影响。
- 时长为 0 或未知时不执行片尾跳过。
- 每集片头 / 片尾各只跳一次；用户手动拖回不重复触发。
- 修改 Go 后运行 `gofmt`；提交前 `go test ./... -count=1`、`go vet ./...`、
  `CGO_ENABLED=1 go build ./...`、前端 `npm test -- --run`、`npm run build` 全绿。
- Wails 绑定由 `mise exec -- env GOCACHE=/tmp/unbox-bindings-cache wails3 generate bindings -f '' -clean=true -ts -i ./...` 生成，不手工编辑。

---

### Task 1: 起播加载反馈

**Files:**
- Modify: `frontend/src/App.vue`
- Modify: `frontend/public/style.css`
- Test: `frontend/src/App.playback.test.ts`

**Interfaces:**
- Consumes: `PlaybackView` 已有的 `playback(state)` 事件、`vodPlaybackStatus`
- Produces: `vodPlayerLoading`（ref<boolean>）；`.player-loading` 加载层

- [ ] **Step 1: 写失败测试**——在 `App.playback.test.ts` 增加源码级断言：`doPlayEpisode` 中
  置 `vodPlayerLoading.value = true`；`onVodPlaybackSignal` 收到 `playing` 时置 false；
  模板包含 `class="player-loading"`。
- [ ] **Step 2: 运行确认失败**：`cd frontend && npm test -- --run src/App.playback.test.ts`
- [ ] **Step 3: 实现**：新增 `vodPlayerLoading` ref；`doPlayEpisode` 开始时置 true，
  `onVodPlaybackSignal` 在 `playing` / `error` 时置 false，`stopPlayback('vod')` 时置 false；
  在 `.vod-player` 内加加载层（半透明遮罩 + 文案「正在加载剧集…」），仅 Web 计划显示。
- [ ] **Step 4: 跑测试确认通过**：同 Step 2 命令 + `npm run build`
- [ ] **Step 5: 提交**：`git commit -m "feat(vod): show loading overlay until first frame"`

---

### Task 2: 跳过标记持久化

**Files:**
- Modify: `internal/shell/service.go`、`internal/shell/service_test.go`
- 生成：`frontend/bindings/.../shellservice.ts`（由绑定命令更新）

**Interfaces:**
- Produces:
  - `type VodSkipMarks struct { IntroEnd float64; OutroStart float64 }`
  - `func (s *ShellService) GetVodSkipMarks(site, vodID string) VodSkipMarks`
  - `func (s *ShellService) SetVodSkipMarks(site, vodID string, marks VodSkipMarks) error`
- 键：`vod.skip.<site>.<vodID>`，值 `{"IntroEnd":..,"OutroStart":..}`

- [ ] **Step 1: 写失败测试**——`TestVodSkipMarksRoundTripAndDefaults`（缺失 → 零值；
  写入 → 新实例读回一致）、`TestVodSkipMarksInvalidJSONFallsBackToZero`（非法 JSON → 零值）、
  `TestVodSkipMarksStoreUnavailable`（store 关闭 → 读取零值、写入报错）。
- [ ] **Step 2: 运行确认失败**：`go test ./internal/shell -run TestVodSkipMarks -count=1`
- [ ] **Step 3: 实现**：定义结构与两个方法；键拼接 `fmt.Sprintf("vod.skip.%s.%s", site, vodID)`；
  读取用 `store.GetKV` + `json.Unmarshal`，任何错误返回零值；写入 `json.Marshal` 后 `SetKV`。
- [ ] **Step 4: 跑测试 + 生成绑定**：`go test ./internal/shell -count=1`，再执行绑定命令，
  确认 `shellservice.ts` 出现两个方法。
- [ ] **Step 5: 提交**：`git commit -m "feat(vod): persist skip-intro marks per series"`

---

### Task 3: 跳过判定纯逻辑

**Files:**
- Create: `frontend/src/vodSkip.ts`
- Test: `frontend/src/vodSkip.test.ts`

**Interfaces:**
- Produces:
  - `interface VodSkipMarks { IntroEnd: number; OutroStart: number }`
  - `interface SkipRuntime { introSkipped: boolean; outroSkipped: boolean }`
  - `resolveSkipAction(position, duration, marks, runtime): 'intro' | 'outro' | null`

- [ ] **Step 1: 写失败测试**——覆盖：未标记返回 null；`position < IntroEnd` 且未跳片头返回
  `'intro'`；已跳片头返回 null；`OutroStart > 0 && duration > 0 && position >= OutroStart`
  返回 `'outro'`；时长未知（0）不返回 outro；已跳片尾不重复返回。
- [ ] **Step 2: 运行确认失败**：`cd frontend && npm test -- --run src/vodSkip.test.ts`
- [ ] **Step 3: 实现**：按上述规则返回动作，片头优先于片尾。
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: 提交**：`git commit -m "feat(vod): add skip-intro decision helper"`

---

### Task 4: 播放器跳过按钮与 App 接入

**Files:**
- Modify: `frontend/src/components/PlaybackView.vue`、`frontend/src/App.vue`、`frontend/public/style.css`
- Test: `frontend/src/components/PlaybackView.test.ts`、`frontend/src/App.playback.test.ts`

**Interfaces:**
- Consumes: Task 2 的绑定、Task 3 的 `resolveSkipAction`
- Produces: `PlaybackView` 新增 props `skipMarks`，emits `markIntro` / `markOutro` / `clearSkipMarks`；
  App 中的 `vodSkipMarks`、`applySkipMarks`、`markIntro` / `markOutro` / `clearSkipMarks`、跳过提示

- [ ] **Step 1: 写失败测试**——组件测试：`.skip-btn` 存在；点开后菜单含三个动作；点击分别
  emit 对应事件。App 源码级断言：`resolveSkipAction` 被调用、`applySkipMarks` 在起播后执行、
  提供 `vodSkipMarks` 与 `SetVodSkipMarks` 调用。
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**：PlaybackView 增加按钮与菜单（复用 `.rate-menu` 样式）；App 载入标记、
  在 `progress` / `playback:event` 中调用 `resolveSkipAction` 并执行 seek 或跳结尾，标记动作
  写回后端，跳过时显示「已跳过片头/片尾」提示（1.5 秒）。
- [ ] **Step 4: 跑测试与构建**
- [ ] **Step 5: 提交**：`git commit -m "feat(vod): auto-skip marked intro and outro"`

---

### Task 5: 文档与完整验证

**Files:**
- Modify: `docs/HANDOFF.md`

- [ ] **Step 1: 更新交接文档**——记录加载反馈机制、跳过标记的键与语义、mpv 调研结论
  （无内置能力，应用层实现）、以及"每集只跳一次"的边界。
- [ ] **Step 2: 完整验证**——`gofmt -l .`、`go test ./... -count=1`、`go vet ./...`、
  `CGO_ENABLED=1 go build ./...`、`npm test -- --run`、`npm run build`。
- [ ] **Step 3: 提交**：`git commit -m "docs: record vod loading feedback and skip marks"`
