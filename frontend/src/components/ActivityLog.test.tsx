import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { ActivityLog } from './ActivityLog'
import type { AuditEntry, Transaction } from '../api/client'
import * as client from '../api/client'

const auditEntry: AuditEntry = {
  id: 1,
  operation: 'transfer',
  transaction_id: 'aaaa-1111',
  account_ids: [1, 2],
  amount: 500.00,
  amount_display: '₹500.00',
  currency: 'INR',
  outcome: 'success',
  failure_reason: null,
  created_at: '2024-01-15T10:30:00Z',
}

const completedTx: Transaction = {
  id: 'aaaa-1111', type: 'transfer', status: 'completed',
  reference_id: null, reversal_of_id: null,
  from_account_id: 1, to_account_id: 2,
  amount: 500.00, amount_display: '₹500.00',
  currency: 'INR', failure_reason: null, created_at: '',
}

const reversedTx: Transaction = { ...completedTx, status: 'reversed' }

const failedAudit: AuditEntry = {
  ...auditEntry, id: 2, outcome: 'failure', failure_reason: 'insufficient funds',
}

const failedTx: Transaction = {
  ...completedTx, id: 'bbbb-2222', status: 'failed', failure_reason: 'insufficient funds',
}

function setup(entries: AuditEntry[], txs: Transaction[]) {
  vi.spyOn(client, 'listAuditLog').mockResolvedValue(entries)
  vi.spyOn(client, 'listTransactions').mockResolvedValue(txs)
  return render(<ActivityLog refreshKey={0} onReversed={vi.fn()} />)
}

describe('ActivityLog', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('renders a row for each entry', async () => {
    setup([auditEntry, failedAudit], [completedTx, failedTx])
    await waitFor(() => expect(screen.getAllByRole('row')).toHaveLength(3)) // header + 2
  })

  it('shows operation type badge', async () => {
    setup([auditEntry], [completedTx])
    await waitFor(() => expect(screen.getByText('transfer')).toBeInTheDocument())
  })

  it('shows formatted amount', async () => {
    setup([auditEntry], [completedTx])
    await waitFor(() => expect(screen.getByText('₹500.00')).toBeInTheDocument())
  })

  it('shows outcome badges', async () => {
    setup([auditEntry, failedAudit], [completedTx, failedTx])
    await waitFor(() => {
      expect(screen.getByText('success')).toBeInTheDocument()
      expect(screen.getByText('failure')).toBeInTheDocument()
    })
  })

  it('shows failure reason', async () => {
    setup([failedAudit], [failedTx])
    await waitFor(() => expect(screen.getByText('insufficient funds')).toBeInTheDocument())
  })

  it('shows Reverse button only for completed transfer', async () => {
    setup([auditEntry], [completedTx])
    await waitFor(() => expect(screen.getByRole('button', { name: 'Reverse' })).toBeInTheDocument())
  })

  it('does NOT show Reverse button for reversed transaction', async () => {
    setup([auditEntry], [reversedTx])
    await waitFor(() => screen.getByText('success'))
    expect(screen.queryByRole('button', { name: 'Reverse' })).not.toBeInTheDocument()
  })

  it('does NOT show Reverse button for failed transaction', async () => {
    setup([failedAudit], [failedTx])
    await waitFor(() => screen.getByText('failure'))
    expect(screen.queryByRole('button', { name: 'Reverse' })).not.toBeInTheDocument()
  })

  it('calls reverseTransaction and onReversed on success', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([auditEntry])
    vi.spyOn(client, 'listTransactions').mockResolvedValue([completedTx])
    vi.spyOn(client, 'reverseTransaction').mockResolvedValue({ ...completedTx, type: 'reversal' })

    const onReversed = vi.fn()
    const user = userEvent.setup()
    render(<ActivityLog refreshKey={0} onReversed={onReversed} />)

    await waitFor(() => screen.getByRole('button', { name: 'Reverse' }))
    await user.click(screen.getByRole('button', { name: 'Reverse' }))
    await waitFor(() => expect(onReversed).toHaveBeenCalled())
  })

  it('shows error when reversal fails', async () => {
    vi.spyOn(client, 'listAuditLog').mockResolvedValue([auditEntry])
    vi.spyOn(client, 'listTransactions').mockResolvedValue([completedTx])
    vi.spyOn(client, 'reverseTransaction').mockRejectedValue(
      new client.ApiError('already reversed', 'ALREADY_REVERSED', 409),
    )

    const user = userEvent.setup()
    render(<ActivityLog refreshKey={0} onReversed={vi.fn()} />)

    await waitFor(() => screen.getByRole('button', { name: 'Reverse' }))
    await user.click(screen.getByRole('button', { name: 'Reverse' }))
    await waitFor(() => expect(screen.getByText('already reversed')).toBeInTheDocument())
  })

  it('shows empty state', async () => {
    setup([], [])
    await waitFor(() => expect(screen.getByText('No activity yet')).toBeInTheDocument())
  })

  it('Prev disabled on first page', async () => {
    setup([auditEntry], [completedTx])
    await waitFor(() => screen.getByText('← Prev'))
    expect(screen.getByText('← Prev')).toBeDisabled()
  })

  it('Next disabled when fewer than PAGE_SIZE entries', async () => {
    setup([auditEntry], [completedTx])
    await waitFor(() => screen.getByText('Next →'))
    expect(screen.getByText('Next →')).toBeDisabled()
  })

  it('refetches when refreshKey changes', async () => {
    const auditSpy = vi.spyOn(client, 'listAuditLog').mockResolvedValue([auditEntry])
    vi.spyOn(client, 'listTransactions').mockResolvedValue([completedTx])
    const { rerender } = render(<ActivityLog refreshKey={0} onReversed={vi.fn()} />)
    await waitFor(() => expect(auditSpy).toHaveBeenCalledTimes(1))

    rerender(<ActivityLog refreshKey={1} onReversed={vi.fn()} />)
    await waitFor(() => expect(auditSpy).toHaveBeenCalledTimes(2))
  })
})
