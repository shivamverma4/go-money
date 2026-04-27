import { useEffect, useState } from 'react'
import { listTransactions, reverseTransaction, ApiError, type Transaction } from '../api/client'
import { Card } from './Card'

interface Props {
  refreshKey: number
  onReversed: () => void
}

const PAGE_SIZE = 10

export function TransactionList({ refreshKey, onReversed }: Props) {
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [reversing, setReversing] = useState<string | null>(null)
  const [reverseError, setReverseError] = useState<string | null>(null)
  const [offset, setOffset] = useState(0)

  useEffect(() => {
    setLoading(true)
    listTransactions({ limit: PAGE_SIZE, offset })
      .then((txs) => { setTransactions(txs); setError(null) })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [refreshKey, offset])

  useEffect(() => { setOffset(0) }, [refreshKey])

  async function handleReverse(id: string) {
    setReversing(id)
    setReverseError(null)
    try {
      await reverseTransaction(id)
      onReversed()
    } catch (e) {
      setReverseError(e instanceof ApiError ? e.message : 'Reversal failed')
    } finally {
      setReversing(null)
    }
  }

  function canReverse(tx: Transaction) {
    return tx.type === 'transfer' && tx.status === 'completed'
  }

  if (loading) return <p className="muted">Loading transactions…</p>
  if (error) return <p className="error">{error}</p>

  return (
    <Card title="Transactions">
      {reverseError && <p className="error">{reverseError}</p>}
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Type</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {transactions.length === 0 && (
              <tr>
                <td colSpan={4} className="muted center">No transactions yet</td>
              </tr>
            )}
            {transactions.map((tx) => (
              <tr key={tx.id}>
                <td className="mono small">{tx.id.slice(0, 8)}…</td>
                <td>
                  <span className={`badge badge-type-${tx.type}`}>{tx.type}</span>
                </td>
                <td>
                  <span className={`badge badge-${tx.status}`}>{tx.status}</span>
                  {tx.failure_reason && (
                    <span className="muted small" title={tx.failure_reason}> ⚠</span>
                  )}
                </td>
                <td>
                  {canReverse(tx) && (
                    <button
                      className="btn-small btn-danger"
                      onClick={() => handleReverse(tx.id)}
                      disabled={reversing === tx.id}
                    >
                      {reversing === tx.id ? '…' : 'Reverse'}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="pagination">
        <button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
          ← Prev
        </button>
        <span className="muted small">
          {transactions.length === 0 ? 'No results' : `${offset + 1}–${offset + transactions.length}`}
        </span>
        <button disabled={transactions.length < PAGE_SIZE} onClick={() => setOffset(offset + PAGE_SIZE)}>
          Next →
        </button>
      </div>
    </Card>
  )
}
