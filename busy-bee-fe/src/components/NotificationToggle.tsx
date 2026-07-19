import { useCallback, useEffect, useState } from 'react'

import { getVapidPublicKey, subscribePush, unsubscribePush } from '../services/api/client'
import { getIdToken } from '../services/token'

function urlBase64ToUint8Array(base64: string): Uint8Array<ArrayBuffer> {
  const padding = '='.repeat((4 - (base64.length % 4)) % 4)
  const raw = atob((base64 + padding).replace(/-/g, '+').replace(/_/g, '/'))
  const arr = new Uint8Array(new ArrayBuffer(raw.length))
  for (let i = 0; i < raw.length; i++) arr[i] = raw.charCodeAt(i)
  return arr
}

const isIOS = /iphone|ipad|ipod/i.test(navigator.userAgent)
const isStandalone = window.matchMedia('(display-mode: standalone)').matches

type State = 'unsupported' | 'off' | 'on' | 'busy' | 'denied'

/** 會議提醒通知開關（Web Push 訂閱管理）。 */
export function NotificationToggle() {
  const [state, setState] = useState<State>('busy')

  useEffect(() => {
    void (async () => {
      if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
        setState('unsupported')
        return
      }
      if (Notification.permission === 'denied') {
        setState('denied')
        return
      }
      const reg = await navigator.serviceWorker.ready
      const sub = await reg.pushManager.getSubscription()
      setState(sub ? 'on' : 'off')
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
    } catch {
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

  if (state === 'unsupported') {
    return isIOS && !isStandalone ? (
      <p className="muted small">iOS 需先「加入主畫面」才能開啟會議提醒通知（iOS 16.4+）。</p>
    ) : null
  }
  if (state === 'denied') {
    return <p className="muted small">通知權限已被封鎖，請在瀏覽器設定中允許本站通知。</p>
  }

  return (
    <label className="notif-toggle">
      <input
        type="checkbox"
        checked={state === 'on'}
        disabled={state === 'busy'}
        onChange={(e) => void (e.target.checked ? enable() : disable())}
      />
      會議提醒通知
    </label>
  )
}
