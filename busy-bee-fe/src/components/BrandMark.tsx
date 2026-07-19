import { Hexagon } from 'lucide-react'

/** 蜂巢六角品牌標記。 */
export function BrandMark({ className = 'size-5' }: { className?: string }) {
  return (
    <span className="relative inline-flex items-center justify-center">
      <Hexagon className={`${className} rotate-90 text-accent`} strokeWidth={1.75} />
      <span className="absolute size-[27%] rounded-full bg-accent" />
    </span>
  )
}
