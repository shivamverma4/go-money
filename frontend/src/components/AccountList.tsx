import { type Account, type Customer } from '../api/client'
import { Card } from './Card'

interface Props {
  accounts: Account[]
  customers: Map<number, Customer>
}

export function AccountList({ accounts, customers }: Props) {
  return (
    <Card title="Accounts">
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Customer</th>
              <th>Balance</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {accounts.map((a) => (
              <tr key={a.id}>
                <td className="mono">{a.id}</td>
                <td>{customers.get(a.customer_id)?.name ?? `#${a.customer_id}`}</td>
                <td className="amount">{a.balance_display}</td>
                <td>
                  <span className={`badge badge-${a.status}`}>{a.status}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  )
}
