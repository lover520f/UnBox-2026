import { describe, expect, it } from 'vitest'
import { resolveSkipAction, type SkipRuntime, type VodSkipMarks } from './vodSkip'

const runtime = (introSkipped = false, outroSkipped = false): SkipRuntime => ({ introSkipped, outroSkipped })
const marks = (introEnd = 0, outroStart = 0): VodSkipMarks => ({ IntroEnd: introEnd, OutroStart: outroStart })

describe('resolveSkipAction', () => {
  it('未标记时不做任何跳过', () => {
    expect(resolveSkipAction(30, 1500, marks(), runtime())).toBeNull()
  })

  it('标记了片头且尚未跳过时返回 intro', () => {
    expect(resolveSkipAction(5, 1500, marks(90), runtime())).toBe('intro')
  })

  it('已过片头结束点或已跳过片头时不再返回 intro', () => {
    expect(resolveSkipAction(120, 1500, marks(90), runtime())).toBeNull()
    expect(resolveSkipAction(5, 1500, marks(90), runtime(true))).toBeNull()
  })

  it('到达片尾起点时返回 outro', () => {
    expect(resolveSkipAction(1440, 1500, marks(0, 1430), runtime())).toBe('outro')
  })

  it('时长未知或已跳过片尾时不返回 outro', () => {
    expect(resolveSkipAction(1440, 0, marks(0, 1430), runtime())).toBeNull()
    expect(resolveSkipAction(1440, 1500, marks(0, 1430), runtime(false, true))).toBeNull()
  })
})
