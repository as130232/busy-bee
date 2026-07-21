import { Navigate } from 'react-router-dom'
import type { ReactNode } from 'react'

import { Loader } from './Loader'
import { useAuth } from '../hooks/useAuth'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, initializing } = useAuth()

  if (initializing)
    return (
      <main className="flex min-h-dvh items-center justify-center">
        <Loader />
      </main>
    )
  if (!user) return <Navigate to="/login" replace />
  return children
}
