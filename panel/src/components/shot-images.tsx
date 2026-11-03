// Labelled screenshots side by side, each at most half the row: the label
// as a small caption over the image, a click opens it full size in a new
// tab. An image that fails to load shows its label and url as text, never
// a broken image. Shared by the question card's Before/After rows, any
// message's Before:/After: lines and the task rail's delivery shots.
import { useState } from 'react'

import { cn } from '@/lib/utils'

export type Shot = { label: string; url: string }

const LABEL = 'text-[11px] font-medium tracking-wide text-muted-foreground uppercase'

function ShotFigure({ shot, imageClassName }: { shot: Shot; imageClassName?: string }) {
  const [broken, setBroken] = useState(false)
  return (
    <figure data-image={shot.label} data-broken={broken || undefined} className="flex min-w-0 flex-col gap-1">
      <figcaption className={LABEL}>{shot.label}</figcaption>
      <a
        href={shot.url}
        target="_blank"
        rel="noreferrer"
        title={`Open ${shot.label} full size`}
        className={cn(broken && 'font-mono text-xs break-all text-muted-foreground underline-offset-2 hover:underline')}
      >
        {broken ? (
          shot.url
        ) : (
          <img
            src={shot.url}
            alt={shot.label}
            loading="lazy"
            onError={() => setBroken(true)}
            className={cn('max-h-72 w-full rounded-md border border-border bg-zinc-900 object-contain', imageClassName)}
          />
        )}
      </a>
    </figure>
  )
}

export function ShotImages({
  shots,
  testId = 'shot-images',
  className,
  imageClassName,
}: {
  shots: Shot[]
  testId?: string
  className?: string
  imageClassName?: string
}) {
  if (shots.length === 0) {
    return null
  }
  return (
    <div data-testid={testId} className={cn('grid grid-cols-2 gap-3', className)}>
      {shots.map((shot) => (
        <ShotFigure key={shot.label} shot={shot} imageClassName={imageClassName} />
      ))}
    </div>
  )
}
