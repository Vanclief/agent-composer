// A message's images under its body: small chamfered plates, each a link
// that opens the image full size in a new tab (ADR 0005 "Images").
import type { MessageImage } from '@/api/msg'

export function MessageImages({ images }: { images?: MessageImage[] }) {
  if (!images || images.length === 0) {
    return null
  }
  return (
    <ul data-testid="message-images" aria-label="Images" className="flex flex-wrap gap-2 pt-1">
      {images.map((image) => (
        <li key={image.name}>
          <a
            href={image.url}
            target="_blank"
            rel="noreferrer"
            data-testid="message-image"
            data-name={image.name}
            title={`Open ${image.name} full size`}
            className="block"
          >
            <img
              src={image.url}
              alt={image.name}
              loading="lazy"
              className="block max-h-40 max-w-60 min-w-12 bg-zinc-900 object-contain"
            />
          </a>
        </li>
      ))}
    </ul>
  )
}
