import { useState, type ReactNode } from 'react'

interface Props {
  title: string
  summary?: string
  children: ReactNode
}

export function Card({ title, summary, children }: Props) {
  const [open, setOpen] = useState(true)
  return (
    <div className={`card${open ? '' : ' card-collapsed'}`}>
      <button className="card-header" onClick={() => setOpen((o) => !o)}>
        <span className="card-title">{title}</span>
        <span className="card-header-right">
          {summary && <span className="card-summary">{summary}</span>}
          <span className="chevron">{open ? '▾' : '▸'}</span>
        </span>
      </button>
      {open && children}
    </div>
  )
}
