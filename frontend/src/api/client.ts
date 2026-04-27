const BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const body = await res.json()
  if (!res.ok) {
    throw new ApiError(body.error ?? 'Request failed', body.code ?? 'UNKNOWN', res.status)
  }
  return body as T
}

export class ApiError extends Error {
  code: string
  status: number
  constructor(message: string, code: string, status: number) {
    super(message)
    this.code = code
    this.status = status
  }
}

// ── Types ──────────────────────────────────────────────────────────────────

export interface Customer {
  id: number
  name: string
  email: string
  kyc_status: string
  created_at: string
}

export interface Account {
  id: number
  customer_id: number
  currency: string
  balance: number
  balance_display: string
  status: 'active' | 'inactive' | 'closed'
}

export interface LedgerEntry {
  id: number
  transaction_id: string
  account_id: number
  debit_amount: number
  credit_amount: number
  entry_date: string
}

export interface Transaction {
  id: string
  type: 'transfer' | 'reversal' | 'deposit' | 'withdrawal'
  status: 'completed' | 'failed' | 'reversed'
  reference_id: string | null
  reversal_of_id: string | null
  from_account_id: number | null
  to_account_id: number | null
  amount: number | null
  amount_display: string | null
  currency: string | null
  failure_reason: string | null
  ledger_entries?: LedgerEntry[]
  created_at: string
}

export interface AuditEntry {
  id: number
  operation: string
  transaction_id: string | null
  account_ids: number[] | null
  amount: number | null
  amount_display: string | null
  currency: string
  outcome: 'success' | 'failure'
  failure_reason: string | null
  created_at: string
}

// ── Customers ──────────────────────────────────────────────────────────────

export const listCustomers = () => request<Customer[]>('/customers')

// ── Accounts ───────────────────────────────────────────────────────────────

export const listAccounts = () => request<Account[]>('/accounts')

// ── Transactions ───────────────────────────────────────────────────────────

export const listTransactions = (params?: { limit?: number; offset?: number }) => {
  const qs = new URLSearchParams()
  if (params?.limit)  qs.set('limit',  String(params.limit))
  if (params?.offset) qs.set('offset', String(params.offset))
  const q = qs.toString()
  return request<Transaction[]>(`/transactions${q ? '?' + q : ''}`)
}

export const createTransfer = (
  fromAccountId: number,
  toAccountId: number,
  amount: number,
) =>
  request<Transaction>('/transactions', {
    method: 'POST',
    body: JSON.stringify({
      from_account_id: fromAccountId,
      to_account_id: toAccountId,
      amount,
    }),
  })

export const reverseTransaction = (id: string) =>
  request<Transaction>(`/transactions/${id}/reverse`, { method: 'POST' })

// ── Audit log ──────────────────────────────────────────────────────────────

export const listAuditLog = (params?: { account_id?: number; outcome?: string; limit?: number; offset?: number }) => {
  const qs = new URLSearchParams()
  if (params?.account_id) qs.set('account_id', String(params.account_id))
  if (params?.outcome) qs.set('outcome', params.outcome)
  if (params?.limit) qs.set('limit', String(params.limit))
  if (params?.offset) qs.set('offset', String(params.offset))
  const q = qs.toString()
  return request<AuditEntry[]>(`/audit-log${q ? '?' + q : ''}`)
}
