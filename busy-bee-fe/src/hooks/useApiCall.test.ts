import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

vi.mock('../services/token', () => ({ getIdToken: vi.fn(async () => 'tok-abc') }))

import { useApiCall } from './useApiCall'

describe('useApiCall', () => {
  it('把 idToken 注入為第一個參數並轉呼叫，回傳結果', async () => {
    const { result } = renderHook(() => useApiCall())
    const fn = vi.fn(async (token: string, a: number, b: string) => `${token}:${a}:${b}`)

    const out = await result.current(fn, 5, 'x')

    expect(fn).toHaveBeenCalledWith('tok-abc', 5, 'x')
    expect(out).toBe('tok-abc:5:x')
  })

  it('call 在重新 render 間身分穩定', () => {
    const { result, rerender } = renderHook(() => useApiCall())
    const first = result.current
    rerender()
    expect(result.current).toBe(first)
  })
})
