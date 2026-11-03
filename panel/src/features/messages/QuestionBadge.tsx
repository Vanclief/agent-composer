// Open questions waiting for me, next to a row's unread count: a pink
// outlined marker with the question icon, distinct from the filled unread
// badge. Nothing while none wait.
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

import { useOpenQuestions } from './questions'
import { UnreadBadge } from './UnreadBadge'

export function QuestionBadge({ count, className }: { count: number; className?: string }) {
  if (count === 0) {
    return null
  }
  const Icon = conceptIcons.question
  return (
    <span
      data-testid="question-badge"
      title={count === 1 ? '1 question waiting for you' : `${count} questions waiting for you`}
      className={cn(
        'inline-flex h-5 items-center gap-0.5 rounded-full border px-1.5 font-mono text-xs leading-5 font-semibold',
        conceptColors.question.border,
        conceptColors.question.text,
        'border-pink-400 dark:border-pink-600',
        className,
      )}
    >
      <Icon aria-hidden className="size-3" />
      {count}
    </span>
  )
}

// The unread count and the questions marker at a row's right end.
export function RowBadges({ count, mention, questions }: { count: number; mention: boolean; questions: number }) {
  if (count === 0 && questions === 0) {
    return null
  }
  return (
    <span className="ml-auto flex shrink-0 items-center gap-1">
      <QuestionBadge count={questions} />
      <UnreadBadge count={count} mention={mention} className="ml-0" />
    </span>
  )
}

// The Talk group header: the open questions for me and the unread messages
// in the channels I follow (useFollow), nothing else.
export function TalkBadges({ count, mention }: { count: number; mention: boolean }) {
  const questions = useOpenQuestions()
  return <RowBadges count={count} mention={mention} questions={questions.total} />
}
