import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { AuditLog } from './AuditLog'
import type { AuditEntry } from '../api/client'
import * as client from '../api/client'

function makeEntry(overrides: Partial<AuditEntry> = {}): AuditEntry {
  return {
    id: 1,
    operation: 'transfer',
    transaction_id: 'uuid-1',
    account_ids: [1, 2],
    amount: 500.00,
    amount_display: '₹500.00',
    currency: 'INR',
    outcome: 'success',
    failure_reason: null,
    created_at: '2024-01-15T10:30:00Z',
    ...overrides,
  }
}

describe('AuditLog', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('renders a row for each entry', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([
      makeEntry({ id: 1 }),
      makeEntry({ id: 2 }),
    ])
    render(<AuditLog refreshKey={0} />)
    await waitFor(() => expect(screen.getAllByRole('row')).toHaveLength(3))
  })

  it('shows operation type badge', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([
      makeEntry({ operation: 'transfer' }),
      makeEntry({ id: 2, operation: 'reversal' }),
    ])
    render(<AuditLog refreshKey={0} />)
    await waitFor(() => {
      expect(screen.getByText('transfer')).toBeInTheDocument()
      expect(screen.getByText('reversal')).toBeInTheDocument()
    })
  })

  it('shows formatted amount', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([makeEntry()])
    render(<AuditLog refreshKey={0} />)
    await waitFor(() => expect(screen.getByText('₹500.00')).toBeInTheDocument())
  })

  it('shows success and failure outcome badges', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([
      makeEntry({ id: 1, outcome: 'success' }),
      makeEntry({ id: 2, outcome: 'failure', failure_reason: 'insufficient funds' }),
    ])
    render(<AuditLog refreshKey={0} />)
    await waitFor(() => {
      expect(screen.getByText('success')).toBeInTheDocument()
      expect(screen.getByText('failure')).toBeInTheDocument()
      expect(screen.getByText('insufficient funds')).toBeInTheDocument()
    })
  })

  it('shows empty state when there are no entries', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([])
    render(<AuditLog refreshKey={0} />)
    await waitFor(() => expect(screen.getByText('No entries')).toBeInTheDocument())
  })

  it('Prev button is disabled on the first page', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([makeEntry()])
    render(<AuditLog refreshKey={0} />)
    await waitFor(() => screen.getByText('← Prev'))
    expect(screen.getByText('← Prev')).toBeDisabled()
  })

  it('Next button is disabled when fewer than PAGE_SIZE entries returned', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([makeEntry()])
    render(<AuditLog refreshKey={0} />)
    await waitFor(() => screen.getByText('Next →'))
    expect(screen.getByText('Next →')).toBeDisabled()
  })

  it('clicking Next advances the page and re-fetches', async () => {
    const entries = Array.from({ length: 10 }, (_, i) => makeEntry({ id: i + 1 }))
    const spy = vi.spyOn(client, 'listAuditLog').mockResolvedValue(entries)

    const user = userEvent.setup()
    render(<AuditLog refreshKey={0} />)

    await waitFor(() => screen.getByText('Next →'))
    await user.click(screen.getByText('Next →'))

    await waitFor(() =>
      expect(spy).toHaveBeenCalledWith(expect.objectContaining({ offset: 10 })),
    )
  })

  it('refetches when refreshKey changes', async () => {
    const spy = vi.spyOn(client, 'listAuditLog').mockResolvedValue([makeEntry()])
    const { rerender } = render(<AuditLog refreshKey={0} />)
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))

    rerender(<AuditLog refreshKey={1} />)
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2))
  })
})
