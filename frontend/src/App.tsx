import { useState, useEffect } from 'react'
import { listAccounts, listCustomers, type Account, type Customer } from './api/client'
import { AccountList } from './components/AccountList'
import { TransferForm } from './components/TransferForm'
import { ActivityLog } from './components/ActivityLog'
import './App.css'

export default function App() {
  const [refreshKey, setRefreshKey] = useState(0)
  const [accounts, setAccounts] = useState<Account[]>([])
  const [customers, setCustomers] = useState<Map<number, Customer>>(new Map())

  // Single fetch for both — re-runs whenever a transfer/reversal triggers refresh.
  useEffect(() => {
    Promise.all([listAccounts(), listCustomers()])
      .then(([accs, custs]) => {
        setAccounts(accs)
        setCustomers(new Map(custs.map((c) => [c.id, c])))
      })
      .catch(() => {})
  }, [refreshKey])

  function refresh() {
    setRefreshKey((k) => k + 1)
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>go-money</h1>
        <span className="subtitle">Banking Ledger</span>
      </header>
      <main className="layout">
        <AccountList accounts={accounts} customers={customers} />
        <div className="span-rows">
          <ActivityLog refreshKey={refreshKey} onReversed={refresh} />
        </div>
        <TransferForm accounts={accounts} customers={customers} refreshKey={refreshKey} onSuccess={refresh} />
      </main>
    </div>
  )
}
