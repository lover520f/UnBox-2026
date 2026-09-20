export interface DonationDonor {
  ID: string
  Name: string
  Avatar: string
  Anonymous: boolean
}

export interface DonationLeaderboard {
  UpdatedAt: string
  Donors: DonationDonor[]
}

type UnknownRecord = Record<string, unknown>
type RankedDonor = DonationDonor & { amount: number; index: number }

function asRecord(value: unknown): UnknownRecord | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? value as UnknownRecord
    : null
}

function field(record: UnknownRecord, pascal: string, camel: string): unknown {
  return record[pascal] ?? record[camel]
}

function stringField(record: UnknownRecord, pascal: string, camel: string): string {
  const value = field(record, pascal, camel)
  return typeof value === 'string' ? value.trim() : ''
}

function amountField(record: UnknownRecord): number {
  const value = field(record, 'Amount', 'amount')
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function donorsField(value: UnknownRecord): unknown[] {
  const donors = field(value, 'Donors', 'donors')
  return Array.isArray(donors) ? donors : []
}

export function normalizeLeaderboard(value: unknown): DonationLeaderboard {
  const source = asRecord(value)
  if (!source) return { UpdatedAt: '', Donors: [] }

  const updatedValue = field(source, 'UpdatedAt', 'updatedAt')
  const updatedAt = typeof updatedValue === 'string' ? updatedValue : ''
  const ranked = donorsField(source).map((item, index): RankedDonor => {
    const donor = asRecord(item)
    if (!donor) {
      return { ID: '', Name: '未命名', Avatar: '', Anonymous: false, amount: 0, index }
    }

    const anonymous = field(donor, 'Anonymous', 'anonymous') === true
    if (anonymous) {
      return {
        ID: '',
        Name: '热心网友',
        Avatar: '',
        Anonymous: true,
        amount: amountField(donor),
        index,
      }
    }

    return {
      ID: stringField(donor, 'ID', 'id'),
      Name: stringField(donor, 'Name', 'name') || '未命名',
      Avatar: stringField(donor, 'Avatar', 'avatar'),
      Anonymous: false,
      amount: amountField(donor),
      index,
    }
  })

  ranked.sort((left, right) => {
    if (left.amount !== right.amount) return right.amount - left.amount
    if (left.Name !== right.Name) return left.Name < right.Name ? -1 : 1
    return left.index - right.index
  })

  return {
    UpdatedAt: updatedAt,
    Donors: ranked.map(({ ID, Name, Avatar, Anonymous }) => ({ ID, Name, Avatar, Anonymous })),
  }
}

const RFC3339 = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/

function pad(value: number, length = 2): string {
  return String(value).padStart(length, '0')
}

export function formatUpdatedAt(value: unknown): string {
  if (typeof value !== 'string' || !RFC3339.test(value)) return '未知'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '未知'

  return [
    `${pad(date.getFullYear(), 4)}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`,
    `${pad(date.getHours())}:${pad(date.getMinutes())}`,
  ].join(' ')
}
