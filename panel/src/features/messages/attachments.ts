// Images a post box holds before they are sent: picked from a drop or a
// paste with the browser's own events, previewed by object URL, checked
// against the server's limits, and read as base64 when sent.
import { useEffect, useRef, useState } from 'react'

import { IMAGE_TYPES, MAX_IMAGE_BYTES, MAX_IMAGES, type ImageUpload } from '@/api/msg'

export type Attachment = {
  key: string
  file: File
  url: string
}

let nextKey = 0

// The files of a drop or a paste that are images; anything else is left out.
export function imageFiles(list: FileList | DataTransferItemList | null | undefined): File[] {
  if (!list) {
    return []
  }
  const files: File[] = []
  for (const item of Array.from(list as ArrayLike<File | DataTransferItem>)) {
    const file = item instanceof File ? item : item.kind === 'file' ? item.getAsFile() : null
    if (file && file.type.startsWith('image/')) {
      files.push(file)
    }
  }
  return files
}

// Whether a drag carries files, so the box can say it takes them.
export function dragHasFiles(types: readonly string[]) {
  return types.includes('Files')
}

// Why these attachments cannot be sent, as the Send button's label, or ''.
export function attachmentProblem(attachments: Attachment[]) {
  if (attachments.length > MAX_IMAGES) {
    return `${MAX_IMAGES} images max`
  }
  if (attachments.some((a) => a.file.size > MAX_IMAGE_BYTES)) {
    return '5 MB max'
  }
  if (attachments.some((a) => !IMAGE_TYPES.includes(a.file.type))) {
    return 'PNG, JPEG, GIF or WebP'
  }
  return ''
}

function readBase64(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = String(reader.result)
      resolve(result.slice(result.indexOf(',') + 1))
    }
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

export function toUploads(attachments: Attachment[]): Promise<ImageUpload[]> {
  return Promise.all(attachments.map(async (a) => ({ name: a.file.name || 'image', data: await readBase64(a.file) })))
}

// The box's attachments; object URLs are released on remove, clear and unmount.
export function useAttachments() {
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const current = useRef(attachments)

  useEffect(() => {
    current.current = attachments
  }, [attachments])

  useEffect(() => () => current.current.forEach((a) => URL.revokeObjectURL(a.url)), [])

  function add(files: File[]) {
    if (files.length === 0) {
      return
    }
    const added = files.map((file) => ({ key: `img-${nextKey++}`, file, url: URL.createObjectURL(file) }))
    setAttachments((list) => [...list, ...added])
  }

  function remove(key: string) {
    setAttachments((list) => {
      list.filter((a) => a.key === key).forEach((a) => URL.revokeObjectURL(a.url))
      return list.filter((a) => a.key !== key)
    })
  }

  function clear() {
    setAttachments((list) => {
      list.forEach((a) => URL.revokeObjectURL(a.url))
      return []
    })
  }

  return { attachments, add, remove, clear }
}
