import { useState, useEffect, useRef } from 'react'
import { createTransfer, ApiError, type Account, type Customer } from '../api/client'
import { Card } from './Card'

interface Props {
  accounts: Account[]
  customers: Map<number, Customer>
  refreshKey: number
  onSuccess: () => void
}

export function TransferForm({ accounts, customers, refreshKey, onSuccess }: Props) {
  const [fromId, setFromId] = useState('')
  const [toId, setToId] = useState('')
  const [amount, setAmount] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  // Skip the one refreshKey bump that this form triggered itself so the
  // success message stays visible. Any subsequent change (e.g. a reversal)
  // clears stale feedback as before.
  const skipNextRefresh = useRef(false)
  useEffect(() => {
    if (skipNextRefresh.current) { skipNextRefresh.current = false; return }
    setSuccess(null)
    setError(null)
  }, [refreshKey])

  const activeAccounts = accounts.filter((a) => a.status === 'active')

  function accountLabel(a: Account) {
    return customers.get(a.customer_id)?.name ?? `Account #${a.id}`
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSuccess(null)

    const amountNum = parseFloat(amount)
    if (isNaN(amountNum) || amountNum <= 0) {
      setError('Enter a valid amount greater than 0')
      return
    }
    if (fromId === toId) {
      setError('Source and destination accounts must differ')
      return
    }

    setSubmitting(true)
    try {
      await createTransfer(Number(fromId), Number(toId), amountNum)
      const fromName = customers.get(accounts.find((a) => String(a.id) === fromId)?.customer_id ?? 0)?.name ?? fromId
      const toName   = customers.get(accounts.find((a) => String(a.id) === toId)?.customer_id ?? 0)?.name ?? toId
      setSuccess(`₹${amount} transferred from ${fromName} to ${toName}`)
      setAmount('')
      setFromId('')
      setToId('')
      skipNextRefresh.current = true
      onSuccess()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Transfer failed')
      // Refresh transaction list and audit log even on failure —
      // the backend still records the failed attempt.
      skipNextRefresh.current = true
      onSuccess()
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card title="New Transfer">
      <form onSubmit={handleSubmit} noValidate>
        <div className="field">
          <label htmlFor="from-account">From</label>
          <select id="from-account" value={fromId} onChange={(e) => setFromId(e.target.value)} required>
            <option value="">Select account</option>
            {activeAccounts.map((a) => (
              <option key={a.id} value={a.id}>
                {accountLabel(a)}
              </option>
            ))}
          </select>
        </div>

        <div className="field">
          <label htmlFor="to-account">To</label>
          <select id="to-account" value={toId} onChange={(e) => setToId(e.target.value)} required>
            <option value="">Select account</option>
            {activeAccounts
              .filter((a) => String(a.id) !== fromId)
              .map((a) => (
                <option key={a.id} value={a.id}>
                  {accountLabel(a)}
                </option>
              ))}
          </select>
        </div>

        <div className="field">
          <label htmlFor="amount">Amount (₹)</label>
          <input
            id="amount"
            type="number"
            min="0.01"
            step="0.01"
            placeholder="0.00"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            required
          />
        </div>

        {error && <p className="error">{error}</p>}
        {success && <p className="success">{success}</p>}

        <button type="submit" disabled={submitting}>
          {submitting ? 'Transferring…' : 'Transfer'}
        </button>
      </form>
    </Card>
  )
}
