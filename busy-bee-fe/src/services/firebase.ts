// Firebase 前端設定：此 config 本來就會公開給瀏覽器，不是機密；
// 安全性由後端 ID token 驗證 + email 白名單把關。
import { initializeApp } from 'firebase/app'
import { getAuth, GoogleAuthProvider } from 'firebase/auth'

const firebaseConfig = {
  apiKey: 'AIzaSyC80yK_SHVANqd0LyKMxIO2MVOIdrgDBg8',
  authDomain: 'busy-bee-502710.firebaseapp.com',
  projectId: 'busy-bee-502710',
  storageBucket: 'busy-bee-502710.firebasestorage.app',
  messagingSenderId: '897794325314',
  appId: '1:897794325314:web:8dd60aaa8a840808bba6db',
}

const app = initializeApp(firebaseConfig)

export const auth = getAuth(app)
export const googleProvider = new GoogleAuthProvider()
