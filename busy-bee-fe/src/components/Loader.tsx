import { Hexagon } from 'lucide-react'

/**
 * 品牌化載入動畫：六角標記呼吸 + sonar 雷達環擴散 + 等寬字掃描標籤。
 * 沿用 RecorderPanel 的 sonar/breathe 視覺語彙，讓載入狀態像 app 原生的一部分。
 * size="sm" 用於頁內區塊、"lg"（預設）用於整頁載入。
 */
export function Loader({
  label = '載入中',
  size = 'lg',
  className = '',
}: {
  label?: string
  size?: 'sm' | 'lg'
  className?: string
}) {
  const box = size === 'lg' ? 'size-24' : 'size-16'
  const ring = size === 'lg' ? 'size-16' : 'size-11'
  const glow = size === 'lg' ? 'size-20' : 'size-14'
  const hex = size === 'lg' ? 'size-11' : 'size-8'

  return (
    <div
      className={`flex flex-col items-center justify-center gap-5 ${className}`}
      role="status"
      aria-label={label}
    >
      <div className={`relative flex ${box} items-center justify-center`}>
        {/* 雷達脈衝環：三環錯開時間，營造向外掃描的科技感 */}
        <span className={`animate-sonar absolute ${ring} rounded-full border border-accent/40`} />
        <span
          className={`animate-sonar absolute ${ring} rounded-full border border-accent/40 [animation-delay:0.9s]`}
        />
        <span
          className={`animate-sonar absolute ${ring} rounded-full border border-accent/40 [animation-delay:1.8s]`}
        />
        {/* 光暈 */}
        <span className={`animate-breathe absolute ${glow} rounded-full bg-accent/20 blur-2xl`} />
        {/* 呼吸中的蜂巢六角標記 */}
        <span className="animate-breathe relative inline-flex items-center justify-center">
          <Hexagon className={`${hex} rotate-90 text-accent`} strokeWidth={1.75} />
          <span className="absolute size-[27%] rounded-full bg-accent" />
        </span>
      </div>

      {/* 等寬字標籤 + 依序閃爍的點，像處理中的終端機輸出 */}
      <span className="flex items-center font-mono text-xs tracking-[0.25em] text-muted">
        {label}
        <span className="animate-loader-dot ml-1">.</span>
        <span className="animate-loader-dot [animation-delay:0.2s]">.</span>
        <span className="animate-loader-dot [animation-delay:0.4s]">.</span>
      </span>
    </div>
  )
}
