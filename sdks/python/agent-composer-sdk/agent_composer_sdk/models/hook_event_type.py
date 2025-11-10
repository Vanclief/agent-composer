from enum import Enum


class HookEventType(str, Enum):
    CONTEXT_EXCEEDED = "context_exceeded"
    CONVERSATION_ENDED = "conversation_ended"
    CONVERSATION_STARTED = "conversation_started"
    POST_CONTEXT_COMPACTION = "post_context_compaction"
    POST_TOOL_USE = "post_tool_use"
    PRE_CONTEXT_COMPACTION = "pre_context_compaction"
    PRE_TOOL_USE = "pre_tool_use"

    def __str__(self) -> str:
        return str(self.value)
