import { useCallback } from 'react'

import { getIdToken } from '../services/token'

/**
 * 統一注入 idToken 的 API 呼叫封裝：把散落的 `fn(await getIdToken(), ...args)`
 * 收斂成 `call(fn, ...args)`，消除各元件重複的取 token 樣板（見重構計畫 P4）。
 * 401／token 過期的集中處理已在 client.ts 的 setUnauthorizedHandler 完成，此處只負責取 token。
 */
export function useApiCall() {
  return useCallback(
    async <Args extends unknown[], R>(
      fn: (idToken: string, ...args: Args) => Promise<R>,
      ...args: Args
    ): Promise<R> => {
      const token = await getIdToken()
      return fn(token, ...args)
    },
    [],
  )
}
