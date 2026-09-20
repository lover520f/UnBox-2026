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

function donorsField(value: UnknownRecord): unknown[] {
  const donors = field(value, 'Donors', 'donors')
  return Array.isArray(donors) ? donors : []
}

export function normalizeLeaderboard(value: unknown): DonationLeaderboard {
  const source = asRecord(value)
  if (!source) return { UpdatedAt: '', Donors: [] }

  const updatedValue = field(source, 'UpdatedAt', 'updatedAt')
  const updatedAt = typeof updatedValue === 'string' ? updatedValue : ''
  const normalizedUpdatedAt = updatedAt === '' || formatUpdatedAt(updatedAt) !== '未知'
    ? updatedAt
    : '未知'
  const donors = donorsField(source).map((item): DonationDonor => {
    const donor = asRecord(item)
    if (!donor) {
      return { ID: '', Name: '热心网友', Avatar: '', Anonymous: false }
    }

    const anonymous = field(donor, 'Anonymous', 'anonymous') === true
    if (anonymous) {
      return {
        ID: '',
        Name: '热心网友',
        Avatar: '',
        Anonymous: true,
      }
    }

    return {
      ID: stringField(donor, 'ID', 'id'),
      Name: stringField(donor, 'Name', 'name') || '热心网友',
      Avatar: stringField(donor, 'Avatar', 'avatar'),
      Anonymous: false,
    }
  })

  return {
    UpdatedAt: normalizedUpdatedAt,
    Donors: donors,
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
