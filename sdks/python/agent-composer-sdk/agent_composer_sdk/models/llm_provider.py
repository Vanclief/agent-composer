from enum import Enum


class LLMProvider(str, Enum):
    OPEN_AI = "open_ai"

    def __str__(self) -> str:
        return str(self.value)
