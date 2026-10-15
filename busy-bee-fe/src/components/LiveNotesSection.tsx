import { NotebookPen } from 'lucide-react'

import type { LiveNotes } from '../services/api/client'
import { CollapsibleSection } from './CollapsibleSection'

/** 即時會議的「會議中筆記」：最後一版即時重點與會議中提問紀錄（F-LIVE）。 */
export function LiveNotesSection({ notes }: { notes: LiveNotes }) {
  const empty = !notes.notes.current && notes.notes.points.length === 0 && notes.qa.length === 0
  if (empty) return null
  return (
    <CollapsibleSection title="會議中筆記" icon={<NotebookPen className="size-4" />} defaultOpen={false}>
      <div className="flex flex-col gap-4 text-sm leading-6">
        {notes.notes.points.length > 0 && (
          <div>
            <h3 className="m-0 mb-1 text-xs font-medium text-muted">即時重點</h3>
            <ol className="m-0 flex flex-col gap-1 pl-5">
              {notes.notes.points.map((p, i) => (
                <li key={i}>{p}</li>
              ))}
            </ol>
          </div>
        )}
        {notes.qa.length > 0 && (
          <div>
            <h3 className="m-0 mb-1 text-xs font-medium text-muted">會議中提問</h3>
            <ul className="m-0 flex list-none flex-col gap-2 p-0">
              {notes.qa.map((q, i) => (
                <li key={i}>
                  <p className="m-0 font-medium">Q：{q.question}</p>
                  <p className="m-0 whitespace-pre-wrap">{q.answer}</p>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </CollapsibleSection>
  )
}
