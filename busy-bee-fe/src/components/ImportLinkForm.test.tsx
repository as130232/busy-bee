import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

vi.mock('../services/api/client', () => ({ importMeeting: vi.fn(async () => ({ meeting: { id: 'm1' } })) }))
vi.mock('../services/token', () => ({ getIdToken: vi.fn(async () => 'tok') }))

import { importMeeting } from '../services/api/client'
import { ImportLinkForm } from './ImportLinkForm'

describe('ImportLinkForm', () => {
  it('initialUrl 預填連結，按下匯入才送出', async () => {
    render(<ImportLinkForm initialUrl="https://youtu.be/abc" />)
    expect(screen.getByPlaceholderText(/貼上 YouTube/)).toHaveValue('https://youtu.be/abc')
    expect(importMeeting).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: /貼連結匯入/ }))
    expect(importMeeting).toHaveBeenCalledWith('tok', {
      url: 'https://youtu.be/abc',
      scenario: 'meeting',
      language: 'zh-TW',
    })
  })
})
