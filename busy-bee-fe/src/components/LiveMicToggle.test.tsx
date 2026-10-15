import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const updatePreferences = vi.fn()
vi.mock('../services/api/client', () => ({ updatePreferences: (...a: unknown[]) => updatePreferences(...a) }))
vi.mock('../services/token', () => ({ getIdToken: async () => 'id' }))

const updateUser = vi.fn()
let enabled = false
vi.mock('../hooks/useAuth', () => ({
  useAuth: () => ({ user: { liveMicTranscription: enabled }, updateUser }),
}))

import { LiveMicToggle } from './LiveMicToggle'

beforeEach(() => {
  updatePreferences.mockReset()
  updateUser.mockReset()
  enabled = false
})

describe('LiveMicToggle', () => {
  it('切換後寫入後端並同步使用者狀態', async () => {
    updatePreferences.mockResolvedValue({ liveMicTranscription: true })
    render(<LiveMicToggle />)
    const sw = screen.getByRole('switch', { name: /即時辨識我的麥克風/ })
    expect(sw).toHaveAttribute('aria-checked', 'false')
    await userEvent.click(sw)
    expect(updatePreferences).toHaveBeenCalledWith('id', { liveMicTranscription: true })
    await waitFor(() => expect(updateUser).toHaveBeenCalledWith({ liveMicTranscription: true }))
  })

  it('失敗時顯示錯誤、不改狀態', async () => {
    updatePreferences.mockRejectedValue(new Error('網路連線失敗'))
    render(<LiveMicToggle />)
    await userEvent.click(screen.getByRole('switch'))
    expect(await screen.findByText('網路連線失敗')).toBeInTheDocument()
    expect(updateUser).not.toHaveBeenCalled()
  })
})
