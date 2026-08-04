import { lazy, Suspense } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { Loader } from './components/Loader'
import { RequireAuth } from './components/RequireAuth'
import { TabLayout } from './components/TabLayout'
import { AuthProvider } from './hooks/useAuth'
import { RecordPage } from './pages/RecordPage'

// RecordPage 是首屏關鍵路徑（start_url='/'），靜態載入；其餘頁面按路由動態切分，
// 縮小主 bundle、加速行動端首屏（見重構計畫 P1）。
const LoginPage = lazy(() => import('./pages/LoginPage').then((m) => ({ default: m.LoginPage })))
const MeetingsPage = lazy(() =>
  import('./pages/MeetingsPage').then((m) => ({ default: m.MeetingsPage })),
)
const AskPage = lazy(() => import('./pages/AskPage').then((m) => ({ default: m.AskPage })))
const SchedulePage = lazy(() =>
  import('./pages/SchedulePage').then((m) => ({ default: m.SchedulePage })),
)
const SettingsPage = lazy(() =>
  import('./pages/SettingsPage').then((m) => ({ default: m.SettingsPage })),
)
const MeetingDetailPage = lazy(() =>
  import('./pages/MeetingDetailPage').then((m) => ({ default: m.MeetingDetailPage })),
)

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Suspense fallback={<Loader className="min-h-dvh" />}>
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
              <Route path="/ask" element={<AskPage />} />
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
        </Suspense>
      </BrowserRouter>
    </AuthProvider>
  )
}
