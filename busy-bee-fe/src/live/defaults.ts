// 即時會議的實際依賴組裝（瀏覽器 API、後端 API、IndexedDB）；測試以假物件注入 LiveSession。
import {
  addMeetingContextFile,
  addMeetingContextText,
  askLive,
  completeUpload,
  deleteMeeting,
  finishLive,
  getLiveToken,
  sendLiveHeartbeat,
  startLiveMeeting,
  updateLiveNotes,
} from '../services/api/client'
import { getIdToken } from '../services/token'
import { putWithProgress } from '../services/upload'
import { MeetTabSource, startMixedRecorder, startPcmCapture } from './audio'
import { RecordingStore } from './recordingStore'
import type { RecoveryDeps } from './recovery'
import type { SessionDeps } from './session'
import { LiveTranscriber } from './transcriber'

export const recordingStore = new RecordingStore()

export function defaultSessionDeps(): SessionDeps {
  return {
    getIdToken,
    api: {
      startLiveMeeting,
      getLiveToken,
      sendLiveHeartbeat,
      updateLiveNotes,
      askLive,
      finishLive,
      completeUpload,
      addMeetingContextText,
      addMeetingContextFile,
    },
    createSource: () => new MeetTabSource(),
    startPcm: startPcmCapture,
    startRecorder: startMixedRecorder,
    createTranscriber: (opts) => new LiveTranscriber(opts),
    store: recordingStore,
    upload: putWithProgress,
  }
}

export function defaultRecoveryDeps(): RecoveryDeps {
  return {
    getIdToken,
    store: recordingStore,
    api: { finishLive, completeUpload, deleteMeeting },
    upload: putWithProgress,
  }
}
