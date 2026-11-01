// PLACEHOLDER owned by the create-task feature: replace freely, keep the
// export and its props (TasksPage opens it).
import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'

import { useCreateTask, useSecrets, useSpecs } from '@/api/hooks'
import { conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { IconChip } from '@/components/icon-chip'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'

type CreateTaskDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CreateTaskDialog({ open, onOpenChange }: CreateTaskDialogProps) {
  const secrets = useSecrets()
  const specs = useSpecs()
  const create = useCreateTask()
  const navigate = useNavigate()
  const [title, setTitle] = useState('')

  function submit(event: FormEvent) {
    event.preventDefault()
    create.mutate(
      { title, spec: {} },
      {
        onSuccess: (task) => {
          onOpenChange(false)
          navigate(`/tasks/${encodeURIComponent(task.name)}`)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>CreateTaskDialog</DialogTitle>
          </DialogHeader>
          <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Review facebook ads" />
          <div className="flex flex-wrap gap-1.5">
            {specs.data?.map((spec) => (
              <IconChip key={spec.name} icon={conceptIcons.savedSpec}>
                {spec.name}
              </IconChip>
            ))}
            {secrets.data?.map((secret) => (
              <IconChip key={secret.name} icon={conceptIcons.secret}>
                {secret.name}
              </IconChip>
            ))}
          </div>
          <ErrorText error={create.error} />
          <DialogFooter>
            <Button type="submit" disabled={!title.trim() || create.isPending}>
              {title.trim() ? 'Create' : 'Create: needs a title'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
