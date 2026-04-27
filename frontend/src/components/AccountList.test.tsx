import { render, screen } from '@testing-library/react'
import { AccountList } from './AccountList'
import type { Account, Customer } from '../api/client'

const customers: Map<number, Customer> = new Map([
  [1, { id: 1, name: 'Alice',   email: 'alice@example.com', kyc_status: 'verified', created_at: '' }],
  [2, { id: 2, name: 'Bob',     email: 'bob@example.com',   kyc_status: 'verified', created_at: '' }],
  [3, { id: 3, name: 'Charlie', email: 'charlie@example.com', kyc_status: 'verified', created_at: '' }],
])

const accounts: Account[] = [
  { id: 1, customer_id: 1, currency: 'INR', balance_subunits: 1000000, balance_display: '₹10,000.00', status: 'active' },
  { id: 2, customer_id: 2, currency: 'INR', balance_subunits: 500000,  balance_display: '₹5,000.00',  status: 'active' },
  { id: 3, customer_id: 3, currency: 'INR', balance_subunits: 0,       balance_display: '₹0.00',       status: 'inactive' },
]

describe('AccountList', () => {
  it('renders a row for each account', () => {
    render(<AccountList accounts={accounts} customers={customers} />)
    expect(screen.getAllByRole('row')).toHaveLength(accounts.length + 1) // +1 for header
  })

  it('shows customer names from the map', () => {
    render(<AccountList accounts={accounts} customers={customers} />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
    expect(screen.getByText('Charlie')).toBeInTheDocument()
  })

  it('displays formatted balances', () => {
    render(<AccountList accounts={accounts} customers={customers} />)
    expect(screen.getByText('₹10,000.00')).toBeInTheDocument()
    expect(screen.getByText('₹5,000.00')).toBeInTheDocument()
  })

  it('shows the correct status badge for each account', () => {
    render(<AccountList accounts={accounts} customers={customers} />)
    expect(screen.getAllByText('active')).toHaveLength(2)
    expect(screen.getByText('inactive')).toBeInTheDocument()
  })

  it('falls back to #id when customer is not in the map', () => {
    render(<AccountList accounts={accounts} customers={new Map()} />)
    expect(screen.getByText('#1')).toBeInTheDocument()
  })

  it('renders empty table body with no accounts', () => {
    render(<AccountList accounts={[]} customers={customers} />)
    expect(screen.queryByRole('row', { name: /alice/i })).not.toBeInTheDocument()
  })
})
