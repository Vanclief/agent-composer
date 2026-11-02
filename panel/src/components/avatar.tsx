// A participant's picture: a deterministic identicon drawn from the name
// (same name, same picture, everywhere), in the spirit of MetaMask's
// jazzicon. A thin ring says who it is: human pink, agent or task violet.
// agc, the reserved system participant, gets a neutral grey mark instead.
// With `presence` set, a PresenceDot sits at the bottom-right corner.
import { useId } from 'react'

import { cn } from '@/lib/utils'

export type AvatarSize = 'xs' | 'sm' | 'md' | 'lg'

const PX: Record<AvatarSize, number> = { xs: 16, sm: 20, md: 28, lg: 40 }

// The panel's concept hues (teal, cyan, blue, indigo, violet, purple,
// fuchsia, pink, rose-pink). Emerald, sky, amber and red belong to
// statuses, so an avatar never borrows them.
const HUES = [172, 188, 204, 226, 252, 272, 292, 316, 338]

// FNV-1a, 32 bits.
export function hashName(name: string) {
  let h = 0x811c9dc5
  for (const ch of name) {
    h ^= ch.codePointAt(0) ?? 0
    h = Math.imul(h, 0x01000193)
  }
  return h >>> 0
}

// mulberry32: a small seeded generator, so every draw follows from the name.
function rng(seed: number) {
  let a = seed
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

type Shape = { kind: 'circle' | 'rect'; x: number; y: number; size: number; turn: number; fill: string }

// A disc color and three or four shapes, circles and rounded rectangles,
// each rotated about the center. Coordinates are on a 40x40 box.
export function identicon(name: string) {
  const seed = hashName(name)
  const next = rng(seed)
  const pick = () => HUES[Math.floor(next() * HUES.length)]
  const color = (hue: number, light: number) => `hsl(${hue} ${55 + Math.floor(next() * 25)}% ${light}%)`
  const disc = color(pick(), 40 + Math.floor(next() * 16))
  const count = 3 + Math.floor(next() * 2)
  const shapes: Shape[] = []
  for (let i = 0; i < count; i++) {
    shapes.push({
      kind: next() < 0.5 ? 'circle' : 'rect',
      x: 6 + Math.round(next() * 28),
      y: 6 + Math.round(next() * 28),
      size: 12 + Math.round(next() * 16),
      turn: Math.round(next() * 360),
      fill: color(pick(), 34 + Math.floor(next() * 38)),
    })
  }
  return { seed: seed.toString(16).padStart(8, '0'), disc, shapes }
}

const RING = {
  human: 'stroke-pink-500 dark:stroke-pink-400',
  agent: 'stroke-violet-500 dark:stroke-violet-400',
}

type AvatarProps = {
  name: string
  size?: AvatarSize
  // Pink ring when true, violet when false (agent or task).
  human?: boolean
  // Shows the presence dot when set.
  presence?: boolean
  className?: string
}

export function Avatar({ name, size = 'sm', human = false, presence, className }: AvatarProps) {
  const clip = useId()
  const px = PX[size]
  const agc = name === 'agc'
  const art = identicon(name)
  return (
    <span
      data-avatar={name}
      data-kind={agc ? 'agc' : human ? 'human' : 'agent'}
      className={cn('relative inline-flex shrink-0', className)}
      style={{ width: px, height: px }}
    >
      <svg viewBox="0 0 40 40" width={px} height={px} role="img" aria-label={`@${name}`} data-seed={art.seed}>
        {agc ? (
          <g data-shapes="">
            <circle cx="20" cy="20" r="20" className="fill-zinc-400 dark:fill-zinc-600" />
            <rect x="12" y="12" width="16" height="16" rx="4" className="fill-zinc-100 dark:fill-zinc-300" />
            <circle cx="20" cy="20" r="3.5" className="fill-zinc-500" />
          </g>
        ) : (
          <>
            <clipPath id={clip}>
              <circle cx="20" cy="20" r="20" />
            </clipPath>
            <g clipPath={`url(#${clip})`} data-shapes="">
              <rect width="40" height="40" fill={art.disc} />
              {art.shapes.map((s, i) =>
                s.kind === 'circle' ? (
                  <circle
                    key={i}
                    cx={s.x}
                    cy={s.y}
                    r={s.size / 2}
                    fill={s.fill}
                    transform={`rotate(${s.turn} 20 20)`}
                  />
                ) : (
                  <rect
                    key={i}
                    x={s.x - s.size / 2}
                    y={s.y - s.size / 2}
                    width={s.size}
                    height={s.size * 0.7}
                    rx={s.size / 5}
                    fill={s.fill}
                    transform={`rotate(${s.turn} 20 20)`}
                  />
                ),
              )}
            </g>
            <circle
              cx="20"
              cy="20"
              r="19"
              fill="none"
              strokeWidth={px <= 20 ? 3 : 2}
              className={human ? RING.human : RING.agent}
            />
          </>
        )}
      </svg>
      {presence !== undefined && (
        <PresenceDot online={presence} size={size} className="absolute -right-0.5 -bottom-0.5" />
      )}
    </span>
  )
}

const DOT: Record<AvatarSize, string> = { xs: 'size-1.5', sm: 'size-2', md: 'size-2.5', lg: 'size-3' }

// Online: a filled emerald dot (tier lit, like a running status). Offline:
// a hollow grey circle (tier off). A background ring keeps it apart from
// the avatar under it.
export function PresenceDot({
  online,
  size = 'sm',
  className,
}: {
  online: boolean
  size?: AvatarSize
  className?: string
}) {
  return (
    <span
      data-testid="presence"
      data-online={online}
      data-tier={online ? 'lit' : 'off'}
      aria-label={online ? 'online' : 'offline'}
      className={cn(
        'inline-block shrink-0 rounded-full ring-background',
        size === 'xs' ? 'ring-1' : 'ring-2',
        DOT[size],
        online ? 'bg-emerald-500' : 'border border-zinc-400 bg-background dark:border-zinc-500',
        className,
      )}
    />
  )
}
