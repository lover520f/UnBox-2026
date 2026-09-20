// vodSkip.ts — 跳过片头片尾的纯判定逻辑：Web 与 mpv 共用同一套规则。
export interface VodSkipMarks {
  IntroEnd: number
  OutroStart: number
}

/** 本次播放已经做过的跳过动作，避免重复触发。 */
export interface SkipRuntime {
  introSkipped: boolean
  outroSkipped: boolean
}

export type SkipAction = 'intro' | 'outro' | null

/**
 * resolveSkipAction 判定当前播放位置该不该跳过：片头优先，其次片尾。
 * - 片头：位置还没到 IntroEnd
 * - 片尾：位置已到 OutroStart，且时长已知（直播流时长为 0，不跳）
 * 每个动作每集只做一次，由调用方把 runtime 标记为已跳过。
 */
export function resolveSkipAction(
  position: number,
  duration: number,
  marks: VodSkipMarks,
  runtime: SkipRuntime,
): SkipAction {
  if (!runtime.introSkipped && marks.IntroEnd > 0 && position < marks.IntroEnd) return 'intro'
  if (!runtime.outroSkipped && marks.OutroStart > 0 && duration > 0 && position >= marks.OutroStart) return 'outro'
  return null
}
