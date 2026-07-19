import { useCallback, useEffect, useState } from 'react'
import { ListChecks } from 'lucide-react'

import { ActionItemList } from './ActionItemList'
import { listPendingActionItems, toggleActionItem, type PendingActionItem } from '../services/api/client'
import { getIdToken } from '../services/token'

/** Dashboard 上的跨會議未完成行動項卡；無待辦時不顯示。 */
export function PendingActionItems() {
  const [items, setItems] = useState<PendingActionItem[]>([])

  const load = useCallback(async () => {
    try {
      const { actionItems } = await listPendingActionItems(await getIdToken())
      setItems(actionItems)
    } catch {
      // 待辦卡為輔助資訊，載入失敗時靜默略過（不干擾主流程）
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const toggle = async (id: string, done: boolean) => {
    setItems((prev) => prev.filter((it) => it.id !== id)) // 勾選即從未完成清單移除（樂觀）
    try {
      await toggleActionItem(await getIdToken(), id, done)
    } catch {
      void load() // 失敗則重載回滾
    }
  }

  if (items.length === 0) return null

  return (
    <section className="rounded-xl border border-border bg-surface p-4">
      <h2 className="m-0 mb-1 flex items-center gap-2 text-sm font-semibold">
        <ListChecks className="size-4 text-accent" />
        待辦行動項
        <span className="text-muted">{items.length}</span>
      </h2>
      <ActionItemList items={items} onToggle={toggle} showMeeting />
    </section>
  )
}
