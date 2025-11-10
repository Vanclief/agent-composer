from enum import Enum


class ConversationStatus(str, Enum):
    CANCELED = "canceled"
    FAILED = "failed"
    QUEUED = "queued"
    RUNNING = "running"
    SUCCEEDED = "succeeded"

    def __str__(self) -> str:
        return str(self.value)
