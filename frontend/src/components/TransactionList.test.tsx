import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { TransactionList } from './TransactionList'
import type { Transaction } from '../api/client'
import * as client from '../api/client'

const transfer: Transaction = {
  id: 'aaaa-1111', type: 'transfer', status: 'completed',
  reference_id: null, reversal_of_id: null,
  from_account_id: 1, to_account_id: 2,
  amount_subunits: 50000, amount_display: '₹500.00',
  currency: 'INR', failure_reason: null, created_at: '',
}

const reversalTx: Transaction = {
  id: 'bbbb-2222', type: 'reversal', status: 'completed',
  reference_id: null, reversal_of_id: 'aaaa-1111',
  from_account_id: 2, to_account_id: 1,
  amount_subunits: 50000, amount_display: '₹500.00',
  currency: 'INR', failure_reason: null, created_at: '',
}

const reversedTransfer: Transaction = { ...transfer, status: 'reversed' }

const failedTransfer: Transaction = {
  ...transfer, id: 'cccc-3333', status: 'failed',
  failure_reason: 'insufficient funds',
}

function setup(transactions: Transaction[]) {
  vi.spyOn(client, 'listTransactions').mockResolvedValue(transactions)
  return render(<TransactionList refreshKey={0} onReversed={vi.fn()} />)
}

const PAGE_SIZE = 10

describe('TransactionList', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('renders a row for each transaction', async () => {
    setup([transfer, reversalTx])
    await waitFor(() => expect(screen.getAllByRole('row')).toHaveLength(3)) // header + 2
  })

  it('shows type badges', async () => {
    setup([transfer, reversalTx])
    await waitFor(() => {
      expect(screen.getByText('transfer')).toBeInTheDocument()
      expect(screen.getByText('reversal')).toBeInTheDocument()
    })
  })

  it('shows Reverse button only for transfer+completed', async () => {
    setup([transfer, reversalTx, reversedTransfer, failedTransfer])
    await waitFor(() => screen.getAllByRole('row'))
    // Only the one completed transfer qualifies.
    expect(screen.getAllByRole('button', { name: 'Reverse' })).toHaveLength(1)
  })

  it('does NOT show Reverse button for reversal transactions', async () => {
    setup([reversalTx])
    await waitFor(() => screen.getByText('reversal'))
    expect(screen.queryByRole('button', { name: 'Reverse' })).not.toBeInTheDocument()
  })

  it('does NOT show Reverse button for reversed transfers', async () => {
    setup([reversedTransfer])
    await waitFor(() => screen.getByText('reversed'))
    expect(screen.queryByRole('button', { name: 'Reverse' })).not.toBeInTheDocument()
  })

  it('does NOT show Reverse button for failed transfers', async () => {
    setup([failedTransfer])
    await waitFor(() => screen.getByText('failed'))
    expect(screen.queryByRole('button', { name: 'Reverse' })).not.toBeInTheDocument()
  })

  it('calls reverseTransaction and onReversed on success', async () => {
    vi.spyOn(client, 'listTransactions').mockResolvedValue([transfer])
    vi.spyOn(client, 'reverseTransaction').mockResolvedValue({ ...reversalTx })

    const onReversed = vi.fn()
    const user = userEvent.setup()
    render(<TransactionList refreshKey={0} onReversed={onReversed} />)

    await waitFor(() => screen.getByRole('button', { name: 'Reverse' }))
    await user.click(screen.getByRole('button', { name: 'Reverse' }))

    await waitFor(() => expect(onReversed).toHaveBeenCalled())
  })

  it('shows error message when reversal fails', async () => {
    vi.spyOn(client, 'listTransactions').mockResolvedValue([transfer])
    vi.spyOn(client, 'reverseTransaction').mockRejectedValue(
      new client.ApiError('transaction has already been reversed', 'ALREADY_REVERSED', 409),
    )

    const user = userEvent.setup()
    render(<TransactionList refreshKey={0} onReversed={vi.fn()} />)

    await waitFor(() => screen.getByRole('button', { name: 'Reverse' }))
    await user.click(screen.getByRole('button', { name: 'Reverse' }))

    await waitFor(() =>
      expect(screen.getByText('transaction has already been reversed')).toBeInTheDocument(),
    )
  })

  it('shows empty state when there are no transactions', async () => {
    setup([])
    await waitFor(() =>
      expect(screen.getByText('No transactions yet')).toBeInTheDocument(),
    )
  })

  it('shows warning icon on failed transactions with a reason', async () => {
    setup([failedTransfer])
    await waitFor(() => screen.getByText('failed'))
    expect(screen.getByTitle('insufficient funds')).toBeInTheDocument()
  })

  it('Prev button is disabled on the first page', async () => {
    setup([transfer])
    await waitFor(() => screen.getByText('← Prev'))
    expect(screen.getByText('← Prev')).toBeDisabled()
  })

  it('Next button is disabled when fewer than PAGE_SIZE entries returned', async () => {
    setup([transfer])
    await waitFor(() => screen.getByText('Next →'))
    expect(screen.getByText('Next →')).toBeDisabled()
  })

  it('clicking Next advances the page and re-fetches', async () => {
    const entries = Array.from({ length: PAGE_SIZE }, (_, i) => ({ ...transfer, id: `id-${i}` }))
    const spy = vi.spyOn(client, 'listTransactions').mockResolvedValue(entries)

    const user = userEvent.setup()
    render(<TransactionList refreshKey={0} onReversed={vi.fn()} />)

    await waitFor(() => screen.getByText('Next →'))
    await user.click(screen.getByText('Next →'))

    await waitFor(() =>
      expect(spy).toHaveBeenCalledWith(expect.objectContaining({ offset: PAGE_SIZE })),
    )
  })
})
