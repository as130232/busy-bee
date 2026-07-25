import { useCallback, useEffect, useState } from 'react'

import { getVapidPublicKey, sendTestPush, subscribePush, unsubscribePush } from '../services/api/client'
import { getIdToken } from '../services/token'

function urlBase64ToUint8Array(base64: string): Uint8Array<ArrayBuffer> {
  const padding = '='.repeat((4 - (base64.length % 4)) % 4)
  const raw = atob((base64 + padding).replace(/-/g, '+').replace(/_/g, '/'))
  const arr = new Uint8Array(new ArrayBuffer(raw.length))
  for (let i = 0; i < raw.length; i++) arr[i] = raw.charCodeAt(i)
  return arr
}

const isIOS = /iphone|ipad|ipod/i.test(navigator.userAgent)
const isStandalone =
  window.matchMedia('(display-mode: standalone)').matches ||
  (navigator as Navigator & { standalone?: boolean }).standalone === true

type State = 'unsupported' | 'off' | 'on' | 'busy' | 'denied'

/** 會議提醒通知開關（Web Push 訂閱管理）。 */
export function NotificationToggle() {
  const [state, setState] = useState<State>('busy')
  const [testMsg, setTestMsg] = useState('')

  useEffect(() => {
    void (async () => {
      try {
        if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
          setState('unsupported')
          return
        }
        if (Notification.permission === 'denied') {
          setState('denied')
          return
        }
        // serviceWorker.ready 在 iOS PWA 首次可能遲遲不 resolve；逾時仍讓使用者可點擊嘗試開啟
        const reg = await Promise.race([
          navigator.serviceWorker.ready,
          new Promise<null>((resolve) => setTimeout(() => resolve(null), 3000)),
        ])
        if (!reg) {
          setState('off')
          return
        }
        const sub = await reg.pushManager.getSubscription()
        setState(sub ? 'on' : 'off')
      } catch {
        setState('off')
      }
    })()
  }, [])

  const enable = useCallback(async () => {
    setState('busy')
    try {
      if ((await Notification.requestPermission()) !== 'granted') {
        setState('denied')
        return
      }
      const token = await getIdToken()
      const { publicKey } = await getVapidPublicKey(token)
      const reg = await navigator.serviceWorker.ready
      const sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(publicKey),
      })
      await subscribePush(token, sub.toJSON())
      setState('on')
    } catch (e) {
      console.error('[push] 開啟通知失敗', e)
      setState('off')
    }
  }, [])

  const disable = useCallback(async () => {
    setState('busy')
    try {
      const reg = await navigator.serviceWorker.ready
      const sub = await reg.pushManager.getSubscription()
      if (sub) {
        await unsubscribePush(await getIdToken(), sub.endpoint)
        await sub.unsubscribe()
      }
      setState('off')
    } catch {
      setState('on')
    }
  }, [])

  // debug / demo：立即送測試推播，驗證顯示層是否正常（免等排程）。
  const test = useCallback(async () => {
    setTestMsg('傳送中…')
    try {
      const { delivered } = await sendTestPush(await getIdToken())
      setTestMsg(delivered > 0 ? `已送出（${delivered} 個裝置），留意通知` : '沒有可送達的訂閱')
    } catch (e) {
      console.error('[push] 測試推播失敗', e)
      setTestMsg('傳送失敗，見 console')
    }
  }, [])

  if (state === 'unsupported') {
    return isIOS && !isStandalone ? (
      <p className="m-0 text-xs text-muted">iOS 需先「加入主畫面」才能開啟會議提醒通知（iOS 16.4+）。</p>
    ) : null
  }
  if (state === 'denied') {
    return <p className="m-0 text-xs text-muted">通知權限已被封鎖，請在瀏覽器設定中允許本站通知。</p>
  }

  return (
    <div className="flex flex-col gap-2">
      <label className="flex cursor-pointer items-center gap-2 text-sm select-none">
        <input
          type="checkbox"
          className="peer sr-only"
          checked={state === 'on'}
          disabled={state === 'busy'}
          onChange={(e) => void (e.target.checked ? enable() : disable())}
        />
        <span className="relative h-6 w-10 shrink-0 rounded-full bg-border transition after:absolute after:top-0.5 after:left-0.5 after:size-5 after:rounded-full after:bg-white after:shadow after:transition peer-checked:bg-accent peer-checked:after:translate-x-4 peer-disabled:opacity-50" />
        會議提醒通知
      </label>
      {state === 'on' && (
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => void test()}
            className="rounded-md border border-border px-2 py-1 text-xs text-muted active:opacity-70"
          >
            發送測試通知
          </button>
          {testMsg && <span className="text-xs text-muted">{testMsg}</span>}
        </div>
      )}
    </div>
  )
}
