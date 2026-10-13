import { renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../services/api/client', () => ({ importMeeting: vi.fn() }))
vi.mock('../services/token', () => ({ getIdToken: vi.fn(async () => 'tok') }))

import { importMeeting } from '../services/api/client'
import { useShareImport } from './useShareImport'

const wrapper = (entry: string) =>
  function Wrapper({ children }: { children: ReactNode }) {
    return <MemoryRouter initialEntries={[entry]}>{children}</MemoryRouter>
  }

afterEach(() => {
  vi.clearAllMocks()
})

describe('useShareImport', () => {
  // 稽核 F2：外部連結帶 ?import= 不可自動觸發付費處理，只預填等使用者確認
  it('抽出分享連結但不自動匯入', async () => {
    const { result } = renderHook(() => useShareImport(), {
      wrapper: wrapper('/?import=' + encodeURIComponent('https://youtu.be/abc')),
    })
    await Promise.resolve()
    expect(result.current).toBe('https://youtu.be/abc')
    expect(importMeeting).not.toHaveBeenCalled()
  })

  it('share_target 的 text 參數也能抽出連結', () => {
    const { result } = renderHook(() => useShareImport(), {
      wrapper: wrapper('/?text=' + encodeURIComponent('看這個 https://example.com/a.mp3')),
    })
    expect(result.current).toBe('https://example.com/a.mp3')
  })

  it('私有網段或無連結回空字串', () => {
    const { result } = renderHook(() => useShareImport(), {
      wrapper: wrapper('/?import=' + encodeURIComponent('http://127.0.0.1/x')),
    })
    expect(result.current).toBe('')
  })
})
