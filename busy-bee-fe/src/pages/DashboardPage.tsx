import { useAuth } from '../hooks/useAuth'

export function DashboardPage() {
  const { user, signOut } = useAuth()
  if (!user) return null // RequireAuth 已保證，防禦性判斷

  return (
    <main>
      <header className="topbar">
        <span>🐝 Busy Bee</span>
        <span className="spacer" />
        {user.avatarUrl && <img className="avatar" src={user.avatarUrl} alt="" referrerPolicy="no-referrer" />}
        <span>{user.displayName || user.email}</span>
        <button type="button" onClick={() => void signOut()}>
          登出
        </button>
      </header>
      <section className="center">
        <p>尚無會議紀錄。</p>
        <p className="muted">錄音與上傳功能將在 M1-B 推出。</p>
      </section>
    </main>
  )
}
