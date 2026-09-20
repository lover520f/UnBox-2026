# 点播加载反馈与跳过片头片尾设计

日期：2026-09-20

## 1. 背景

- **加载反馈缺失**：详情页点击剧集后，`doPlayEpisode` 在拿到后端播放计划时就把状态置为
  `playing`，此前的「正在加载剧集…」随之消失；而这时 hls.js 才刚开始拉清单和分片，画面
  尚未出现。真正的等待窗口恰好没有任何反馈，用户会以为点击没生效。
- **需要跳过片头片尾**：用户希望在播放中用按钮标记片头结束 / 片尾开始，之后同一部剧的
  其它集自动跳过。

## 2. mpv 能力调研（mpv 0.37 实测）

mpv **没有内置的跳过片头片尾功能**。可用原语：

| 能力 | 实测结果 | 说明 |
|---|---|---|
| `--start` / `--end` | 选项存在 | `--end` 支持相对时间或百分比；`end` 亦为运行时可设属性 |
| `edl://` | 协议存在 | 可做真实段剪辑，但跳片尾需预先知道总时长 |
| `--scripts` / `--script-opts` | 选项存在 | 社区方案（skip-intro.lua 等）走这条 |

**结论**：UnBox 已通过 JSON IPC 观察 `time-pos` 并能发 `seek`，**在应用层实现最省事**，
Web 与 mpv 共用同一套跳过规则，无需注入 Lua 脚本或构造 EDL。

## 3. 目标

1. 点击剧集后立即给出加载反馈，直到画面真正出现才消失；加载失败给出错误反馈。
2. 播放中可标记片头结束 / 片尾开始，标记按「站点 + 影片」持久化。
3. 同一部剧的其它集自动跳过片头（起播即 seek）与片尾（到达片尾起点即跳结尾）。
4. 跳过行为同时覆盖 Web 与 mpv 两种后端。

## 4. 设计

### 4.1 加载反馈

- `PlaybackView` 已有标准的 `playback` 事件（`playing` / `ready` / `buffering` / `error`）。
- App 维护 `vodPlayerLoading`：开始一次点播播放时置 `true`，收到该会话 token 的
  `playing` 事件（画面真正开始播放）时置 `false`；`error` 与停止播放时同样置 `false`。
- 在 `.vod-player` 内、`PlaybackView` 之上渲染加载层（半透明遮罩 + 旋转指示 + 文案
  「正在加载剧集…」），仅对 Web 计划显示；mpv 计划沿用既有「正在使用 mpv 播放」文案。
- 现有 `vodPlaybackStatus === 'preparing'` 的文案保留（覆盖解析阶段）。

### 4.2 标记的数据模型

- 类型：`VodSkipMarks { IntroEnd float64; OutroStart float64 }`，单位秒，`0` 表示未标记。
- 存储：`store.kv`，键 `vod.skip.<site>.<vodID>`，值为 JSON。
- 绑定：`GetVodSkipMarks(site, vodID) VodSkipMarks`、`SetVodSkipMarks(site, vodID, marks) error`。
  读取失败 / 值非法时按未标记处理，不影响播放。

### 4.3 交互

- 播放器右上角工具栏在「旋转」旁新增「跳过」按钮，点开小菜单（与轨道/倍速菜单同款）：
  - 标记片头结束：记录当前位置为 `IntroEnd`
  - 标记片尾开始：记录当前位置为 `OutroStart`
  - 清除标记
- 菜单里显示当前已标记的时间，便于确认。
- 标记后立即持久化；片尾标记当集即生效，片头标记从下一集起生效并提示用户。

### 4.4 跳过执行

- 判定逻辑收敛为纯函数 `resolveSkipAction(position, duration, marks, state)`，返回
  `'intro' | 'outro' | null`，由 App 调用：
  - **片头**：新集就绪（拿到 `ready`/首次 `playing`）且 `IntroEnd > 0` 时 seek 到 `IntroEnd`；
    每集只做一次。
  - **片尾**：播放位置 `>= OutroStart`（且 `OutroStart > 0`、时长已知、本集未跳过）时跳到结尾，
    触发既有的 `ended` → 自动切集；未开启自动切集时停在结尾。
- Web 用 `progress` 事件驱动；mpv 用已有的 `playback:event` 位置事件驱动，seek 走
  `ShellService.Seek`。
- 跳过时在画面上短暂显示「已跳过片头 / 已跳过片尾」。

### 4.5 边界与限制

- 只对点播详情页生效；直播与本地媒体库不受影响。
- 时长为 0 或未知（直播流）时不执行片尾跳过。
- 用户手动拖回片头不会再次触发跳过（每集只跳一次）。
- 标记为 0 或未标记时不产生任何行为。

## 5. 测试策略

- **Go**：`GetVodSkipMarks` / `SetVodSkipMarks` 的默认值、往返、非法 JSON 归一化、store 不可用。
- **前端纯逻辑**：`resolveSkipAction` 的片头 / 片尾 / 时长未知 / 已跳过 / 无标记分支。
- **组件**：`PlaybackView` 跳过按钮与菜单渲染、三个动作 emit 正确事件。
- **App 接线**：源码级断言（复用既有 `App.playback.test.ts` 风格）。
- 提交前跑 `go test ./... -count=1`、`go vet ./...`、`CGO_ENABLED=1 go build ./...`、
  前端 `npm test -- --run` 与 `npm run build`。

## 6. 非目标

- 不实现基于章节元数据或音频指纹的自动识别片头片尾。
- 不为 mpv 注入 Lua 脚本，也不使用 EDL 剪辑。
- 不按单集存储标记（同一部剧共用一套标记）。
- 不改变直播与本地媒体库的播放策略。
