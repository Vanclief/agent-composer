// The top bar shows the current page's title. Each page sets its own with
// usePageTitle; the layout owns the state.
import { createContext, useContext, useEffect } from 'react'

export const PageTitleContext = createContext<(title: string) => void>(() => {})

export function usePageTitle(title: string) {
  const setTitle = useContext(PageTitleContext)
  useEffect(() => {
    setTitle(title)
    document.title = title ? `${title} · agc` : 'agc'
  }, [title, setTitle])
}
