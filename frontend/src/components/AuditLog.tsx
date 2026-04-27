import { useEffect, useState } from 'react'
import { listAuditLog, type AuditEntry } from '../api/client'
import { Card } from './Card'

interface Props {
  refreshKey: number
}

const PAGE_SIZE = 10

export function AuditLog({ refreshKey }: Props) {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [offset, setOffset] = useState(0)

  useEffect(() => {
    setLoading(true)
    listAuditLog({ limit: PAGE_SIZE, offset })
      .then((data) => { setEntries(data); setError(null) })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [refreshKey, offset])

  function fmt(ts: string) {
    return new Date(ts).toLocaleString('en-IN', { timeZone: 'Asia/Kolkata' })
  }

  if (loading) return <p className="muted">Loading audit log…</p>
  if (error) return <p className="error">{error}</p>

  return (
    <Card title="Audit Log">
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Operation</th>
              <th>Accounts</th>
              <th>Amount</th>
              <th>Outcome</th>
              <th>Reason</th>
            </tr>
          </thead>
          <tbody>
            {entries.length === 0 && (
              <tr>
                <td colSpan={6} className="muted center">No entries</td>
              </tr>
            )}
            {entries.map((e) => (
              <tr key={e.id}>
                <td className="small">{fmt(e.created_at)}</td>
                <td>
                  <span className={`badge badge-type-${e.operation}`}>{e.operation}</span>
                </td>
                <td className="mono small">
                  {e.account_ids?.join(' → ') ?? '—'}
                </td>
                <td className="amount">{e.amount_display ?? '—'}</td>
                <td>
                  <span className={`badge badge-${e.outcome}`}>{e.outcome}</span>
                </td>
                <td className="small muted">{e.failure_reason ?? ''}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="pagination">
        <button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
          ← Prev
        </button>
        <span className="muted small">Showing {offset + 1}–{offset + entries.length}</span>
        <button disabled={entries.length < PAGE_SIZE} onClick={() => setOffset(offset + PAGE_SIZE)}>
          Next →
        </button>
      </div>
    </Card>
  )
}
