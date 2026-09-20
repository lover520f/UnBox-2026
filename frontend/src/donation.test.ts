import { describe, expect, it } from 'vitest'
import { formatUpdatedAt, normalizeLeaderboard } from './donation'

describe('normalizeLeaderboard', () => {
  it('空值或缺少字段时返回空榜', () => {
    expect(normalizeLeaderboard(null)).toEqual({ UpdatedAt: '', Donors: [] })
    expect(normalizeLeaderboard({})).toEqual({ UpdatedAt: '', Donors: [] })
    expect(normalizeLeaderboard({ donors: null })).toEqual({ UpdatedAt: '', Donors: [] })
    expect(normalizeLeaderboard({ donors: [] })).toEqual({ UpdatedAt: '', Donors: [] })
  })

  it('按金额降序、同额按昵称排序，并移除金额字段', () => {
    const leaderboard = normalizeLeaderboard({
      updatedAt: '2026-09-20T12:00:00+08:00',
      donors: [
        { id: 'c', name: '丙', avatar: 'c.png', amount: 50, anonymous: false },
        { id: 'a', name: '甲', avatar: 'a.png', amount: 128, anonymous: false },
        { id: 'b', name: '乙', avatar: 'b.png', amount: 50, anonymous: false },
      ],
    })

    expect(leaderboard.UpdatedAt).toBe('2026-09-20T12:00:00+08:00')
    expect(leaderboard.Donors).toEqual([
      { ID: 'a', Name: '甲', Avatar: 'a.png', Anonymous: false },
      { ID: 'c', Name: '丙', Avatar: 'c.png', Anonymous: false },
      { ID: 'b', Name: '乙', Avatar: 'b.png', Anonymous: false },
    ])
    expect(leaderboard.Donors.every((donor) => !Object.prototype.hasOwnProperty.call(donor, 'Amount'))).toBe(true)
  })

  it('非法金额按 0 处理，同额同名保持输入顺序', () => {
    const leaderboard = normalizeLeaderboard({
      donors: [
        { id: 'first', name: '同名', amount: Number.NaN },
        { id: 'second', name: '同名', amount: '128' },
        { id: 'third', name: '同名', amount: Number.POSITIVE_INFINITY },
      ],
    })

    expect(leaderboard.Donors.map((donor) => donor.ID)).toEqual(['first', 'second', 'third'])
  })

  it('匿名条目不暴露 ID、头像和真实昵称', () => {
    const leaderboard = normalizeLeaderboard({
      donors: [
        {
          id: 'secret-id',
          name: '真实姓名',
          avatar: 'https://example.com/avatar.png',
          amount: 999,
          anonymous: true,
        },
      ],
    })

    expect(leaderboard.Donors).toEqual([
      { ID: '', Name: '热心网友', Avatar: '', Anonymous: true },
    ])
  })

  it('缺失展示字段时提供安全默认值', () => {
    const leaderboard = normalizeLeaderboard({ donors: [{}] })

    expect(leaderboard.Donors).toEqual([
      { ID: '', Name: '未命名', Avatar: '', Anonymous: false },
    ])
  })
})

describe('formatUpdatedAt', () => {
  it('空值或非法值返回未知', () => {
    expect(formatUpdatedAt('')).toBe('未知')
    expect(formatUpdatedAt(null)).toBe('未知')
    expect(formatUpdatedAt('not-a-date')).toBe('未知')
  })

  it('正常 RFC3339 时间返回可读文本', () => {
    const formatted = formatUpdatedAt('2026-09-20T12:34:56+08:00')

    expect(formatted).toMatch(/^2026-09-20 \d{2}:\d{2}$/)
  })
})
