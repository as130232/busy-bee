import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { RequireAuth } from './components/RequireAuth'
import { AuthProvider } from './hooks/useAuth'
import { DashboardPage } from './pages/DashboardPage'
import { LoginPage } from './pages/LoginPage'
import { MeetingDetailPage } from './pages/MeetingDetailPage'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/"
            element={
              <RequireAuth>
                <DashboardPage />
              </RequireAuth>
            }
          />
          <Route
            path="/meetings/:id"
            element={
              <RequireAuth>
                <MeetingDetailPage />
              </RequireAuth>
            }
          />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
