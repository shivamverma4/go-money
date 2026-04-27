import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { TransferForm } from './TransferForm'
import type { Account, Customer } from '../api/client'
import * as client from '../api/client'

const customers: Map<number, Customer> = new Map([
  [1, { id: 1, name: 'Alice', email: 'alice@example.com', kyc_status: 'verified', created_at: '' }],
  [2, { id: 2, name: 'Bob',   email: 'bob@example.com',   kyc_status: 'verified', created_at: '' }],
])

const accounts: Account[] = [
  { id: 1, customer_id: 1, currency: 'INR', balance_subunits: 1000000, balance_display: '₹10,000.00', status: 'active' },
  { id: 2, customer_id: 2, currency: 'INR', balance_subunits: 500000,  balance_display: '₹5,000.00',  status: 'active' },
]

function renderForm(refreshKey = 0, onSuccess = vi.fn()) {
  return render(
    <TransferForm accounts={accounts} customers={customers} refreshKey={refreshKey} onSuccess={onSuccess} />,
  )
}

describe('TransferForm', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('renders From, To and Amount fields', () => {
    renderForm()
    expect(screen.getByLabelText('From')).toBeInTheDocument()
    expect(screen.getByLabelText('To')).toBeInTheDocument()
    expect(screen.getByLabelText('Amount (₹)')).toBeInTheDocument()
  })

  it('populates From dropdown with active account names', () => {
    renderForm()
    const fromSelect = screen.getByLabelText('From')
    expect(within(fromSelect).getByRole('option', { name: 'Alice' })).toBeInTheDocument()
    expect(within(fromSelect).getByRole('option', { name: 'Bob' })).toBeInTheDocument()
  })

  it('excludes the selected From account from the To dropdown', async () => {
    const user = userEvent.setup()
    renderForm()

    await user.selectOptions(screen.getByLabelText('From'), '1')

    const toSelect = screen.getByLabelText('To')
    const optionValues = Array.from(toSelect.querySelectorAll('option')).map((o) => o.value)
    expect(optionValues).not.toContain('1')
    expect(optionValues).toContain('2')
  })

  it('shows validation error for zero amount', async () => {
    const user = userEvent.setup()
    renderForm()

    await user.selectOptions(screen.getByLabelText('From'), '1')
    await user.selectOptions(screen.getByLabelText('To'), '2')
    await user.type(screen.getByLabelText('Amount (₹)'), '0')
    await user.click(screen.getByRole('button', { name: 'Transfer' }))

    expect(screen.getByText(/valid amount/i)).toBeInTheDocument()
  })

  it('calls createTransfer and shows success message on submit', async () => {
    vi.spyOn(client, 'createTransfer').mockResolvedValue({
      id: 'uuid-1', type: 'transfer', status: 'completed',
      reference_id: null, reversal_of_id: null,
      from_account_id: 1, to_account_id: 2,
      amount_subunits: 50000, amount_display: '₹500.00',
      currency: 'INR', failure_reason: null, created_at: '',
    })

    const onSuccess = vi.fn()
    const user = userEvent.setup()
    render(
      <TransferForm accounts={accounts} customers={customers} refreshKey={0} onSuccess={onSuccess} />,
    )

    await user.selectOptions(screen.getByLabelText('From'), '1')
    await user.selectOptions(screen.getByLabelText('To'), '2')
    await user.type(screen.getByLabelText('Amount (₹)'), '500')
    await user.click(screen.getByRole('button', { name: 'Transfer' }))

    await waitFor(() => expect(onSuccess).toHaveBeenCalled())
    expect(screen.getByText(/transferred from Alice to Bob/i)).toBeInTheDocument()
  })

  it('shows API error message on failed transfer and triggers refresh', async () => {
    vi.spyOn(client, 'createTransfer').mockRejectedValue(
      new client.ApiError('insufficient funds', 'INSUFFICIENT_FUNDS', 422),
    )

    const onSuccess = vi.fn()
    const user = userEvent.setup()
    render(
      <TransferForm accounts={accounts} customers={customers} refreshKey={0} onSuccess={onSuccess} />,
    )

    await user.selectOptions(screen.getByLabelText('From'), '1')
    await user.selectOptions(screen.getByLabelText('To'), '2')
    await user.type(screen.getByLabelText('Amount (₹)'), '999999')
    await user.click(screen.getByRole('button', { name: 'Transfer' }))

    await waitFor(() => expect(screen.getByText('insufficient funds')).toBeInTheDocument())
    // Refresh must fire so transaction list / audit log re-fetch the failed record.
    expect(onSuccess).toHaveBeenCalled()
  })

  it('clears success message when refreshKey changes (e.g. after a reversal)', async () => {
    vi.spyOn(client, 'createTransfer').mockResolvedValue({
      id: 'uuid-1', type: 'transfer', status: 'completed',
      reference_id: null, reversal_of_id: null,
      from_account_id: 1, to_account_id: 2,
      amount_subunits: 50000, amount_display: '₹500.00',
      currency: 'INR', failure_reason: null, created_at: '',
    })

    const user = userEvent.setup()
    const { rerender } = render(
      <TransferForm accounts={accounts} customers={customers} refreshKey={0} onSuccess={vi.fn()} />,
    )

    await user.selectOptions(screen.getByLabelText('From'), '1')
    await user.selectOptions(screen.getByLabelText('To'), '2')
    await user.type(screen.getByLabelText('Amount (₹)'), '500')
    await user.click(screen.getByRole('button', { name: 'Transfer' }))
    await waitFor(() => expect(screen.getByText(/transferred from Alice/i)).toBeInTheDocument())

    // refreshKey=1 mirrors the bump the form triggers via onSuccess — message must survive.
    rerender(<TransferForm accounts={accounts} customers={customers} refreshKey={1} onSuccess={vi.fn()} />)
    expect(screen.getByText(/transferred from Alice/i)).toBeInTheDocument()

    // refreshKey=2 simulates a subsequent external action (e.g. a reversal) — message must clear.
    rerender(<TransferForm accounts={accounts} customers={customers} refreshKey={2} onSuccess={vi.fn()} />)
    expect(screen.queryByText(/transferred from Alice/i)).not.toBeInTheDocument()
  })
})
