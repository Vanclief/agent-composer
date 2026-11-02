// Posting as the signed-in human, shared by the composers and the Answer
// box: when agc forgot the stored participant, join again once and retry.
import { usePostMessage } from '@/api/hooks'
import type { PostRequest } from '@/api/msg'

import { isStaleParticipant, type Identity } from './identity'

export function useSend(identity: Identity) {
  const post = usePostMessage()

  // Resolves true once posted; a failure is left in error.
  async function send(request: Omit<PostRequest, 'participant_id'>) {
    try {
      await post.mutateAsync({ ...request, participant_id: identity.participantID })
      return true
    } catch (error) {
      if (!isStaleParticipant(error)) {
        return false
      }
    }
    const id = await identity.rejoin()
    if (!id) {
      return false
    }
    try {
      await post.mutateAsync({ ...request, participant_id: id })
      return true
    } catch {
      return false
    }
  }

  return { send, isPending: post.isPending, error: post.error }
}
