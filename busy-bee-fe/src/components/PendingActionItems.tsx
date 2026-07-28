import { useCallback, useEffect, useState } from 'react'
import { ListChecks } from 'lucide-react'

import { ActionItemList } from './ActionItemList'
import { CollapsibleSection } from './CollapsibleSection'
import {
  deleteActionItem,
  listPendingActionItems,
  toggleActionItem,
  type PendingActionItem,
} from '../services/api/client'
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

  // 刪除誤抽/重複的待辦（跨會議列表不去重，重複項可在此手動清掉）。
  const remove = async (id: string) => {
    setItems((prev) => prev.filter((it) => it.id !== id)) // 樂觀移除
    try {
      await deleteActionItem(await getIdToken(), id)
    } catch {
      void load() // 失敗則重載回滾
    }
  }

  if (items.length === 0) return null

  return (
    <CollapsibleSection
      title="待辦"
      count={items.length}
      icon={<ListChecks className="size-4 text-accent" />}
    >
      <ActionItemList items={items} onToggle={toggle} onRemove={remove} showMeeting />
    </CollapsibleSection>
  )
}
