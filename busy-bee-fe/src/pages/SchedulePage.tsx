import { MeetingList } from '../components/MeetingList'
import { PendingActionItems } from '../components/PendingActionItems'
import { ScheduleForm } from '../components/ScheduleForm'
import { useMeetings } from '../hooks/useMeetings'

/** 行程分頁：排程會議清單 + 新增排程 + 跨會議待辦行動項。 */
export function SchedulePage() {
  const { meetings, reload } = useMeetings()
  // scheduledAt 為空的是上傳流程的暫存記錄（非用戶排程），不顯示
  const scheduled = meetings.filter((m) => m.status === 'scheduled' && m.scheduledAt)

  return (
    <>
      <div className="flex items-center justify-between gap-3">
        <h1 className="m-0 text-base font-semibold">會議行程</h1>
        <ScheduleForm onCreated={reload} />
      </div>

      <PendingActionItems />

      <MeetingList meetings={scheduled} emptyText="尚無排程會議，點右上角新增。" />
    </>
  )
}
