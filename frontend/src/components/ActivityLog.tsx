import { useEffect, useState } from 'react'
import {
  listAuditLog, listTransactions, reverseTransaction,
  ApiError, type AuditEntry, type Transaction,
} from '../api/client'
import { Card } from './Card'

interface Props {
  refreshKey: number
  onReversed: () => void
}

const PAGE_SIZE = 10

export function ActivityLog({ refreshKey, onReversed }: Props) {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [txMap, setTxMap] = useState<Map<string, Transaction>>(new Map())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [reversing, setReversing] = useState<string | null>(null)
  const [reverseError, setReverseError] = useState<string | null>(null)
  const [offset, setOffset] = useState(0)

  useEffect(() => {
    setLoading(true)
    Promise.all([
      listAuditLog({ limit: PAGE_SIZE, offset }),
      listTransactions({ limit: PAGE_SIZE, offset }),
    ])
      .then(([audit, txs]) => {
        setEntries(audit)
        setTxMap(new Map(txs.map((t) => [t.id, t])))
        setError(null)
      })
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

  function canReverse(tx: Transaction | undefined) {
    return tx?.type === 'transfer' && tx?.status === 'completed'
  }

  function fmt(ts: string) {
    return new Date(ts).toLocaleString('en-IN', { timeZone: 'Asia/Kolkata' })
  }

  if (loading) return <p className="muted">Loading activity…</p>
  if (error) return <p className="error">{error}</p>

  return (
    <Card title="Activity">
      {reverseError && <p className="error">{reverseError}</p>}
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Type</th>
              <th>Accounts</th>
              <th>Amount</th>
              <th>Status</th>
              <th>Reason</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {entries.length === 0 && (
              <tr>
                <td colSpan={7} className="muted center">No activity yet</td>
              </tr>
            )}
            {entries.map((e) => {
              const tx = e.transaction_id ? txMap.get(e.transaction_id) : undefined
              return (
                <tr key={e.id}>
                  <td className="small">{fmt(e.created_at)}</td>
                  <td>
                    <span className={`badge badge-type-${e.operation}`}>{e.operation}</span>
                  </td>
                  <td className="mono small">{e.account_ids?.join(' → ') ?? '—'}</td>
                  <td className="amount">{e.amount_display ?? '—'}</td>
                  <td>
                    <span className={`badge badge-${e.outcome}`}>{e.outcome}</span>
                  </td>
                  <td className="small muted">{e.failure_reason ?? ''}</td>
                  <td>
                    {canReverse(tx) && (
                      <button
                        className="btn-small btn-danger"
                        onClick={() => handleReverse(tx!.id)}
                        disabled={reversing === tx!.id}
                      >
                        {reversing === tx!.id ? '…' : 'Reverse'}
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      <div className="pagination">
        <button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
          ← Prev
        </button>
        <span className="muted small">
          {entries.length === 0 ? 'No results' : `${offset + 1}–${offset + entries.length}`}
        </span>
        <button disabled={entries.length < PAGE_SIZE} onClick={() => setOffset(offset + PAGE_SIZE)}>
          Next →
        </button>
      </div>
    </Card>
  )
}
