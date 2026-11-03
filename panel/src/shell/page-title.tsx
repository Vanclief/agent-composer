// The top bar shows the current page's title. Each page sets its own with
// usePageTitle; the layout owns the state.
//
// title is the page word (Messages, Direct, Tasks…), shown uppercase and
// condensed; id is an identifier the page is about (#designer-1,
// @franco/yc-pitch, review-42), shown in monospace, lowercase as sent. The
// browser tab reads "<title> <id> · agc".
import { createContext, useContext, useEffect } from 'react'

export type PageTitle = { title: string; id?: string }

export const PageTitleContext = createContext<(title: PageTitle) => void>(() => {})

export function usePageTitle(title: string, id?: string) {
  const setTitle = useContext(PageTitleContext)
  useEffect(() => {
    setTitle({ title, id })
    const text = [title, id].filter(Boolean).join(' ')
    document.title = text ? `${text} · agc` : 'agc'
  }, [title, id, setTitle])
}
