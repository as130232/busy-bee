import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { RequireAuth } from './components/RequireAuth'
import { TabLayout } from './components/TabLayout'
import { AuthProvider } from './hooks/useAuth'
import { LoginPage } from './pages/LoginPage'
import { MeetingDetailPage } from './pages/MeetingDetailPage'
import { MeetingsPage } from './pages/MeetingsPage'
import { RecordPage } from './pages/RecordPage'
import { SchedulePage } from './pages/SchedulePage'
import { SettingsPage } from './pages/SettingsPage'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            element={
              <RequireAuth>
                <TabLayout />
              </RequireAuth>
            }
          >
            <Route path="/" element={<RecordPage />} />
            <Route path="/meetings" element={<MeetingsPage />} />
            <Route path="/schedule" element={<SchedulePage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
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
