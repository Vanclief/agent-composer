FROM node:22-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends \
    git tmux curl ca-certificates openssh-client jq ripgrep less procps vim-tiny \
    && rm -rf /var/lib/apt/lists/*
RUN npm install -g @anthropic-ai/claude-code @openai/codex
RUN useradd -m -s /bin/bash agent
USER agent
WORKDIR /home/agent
# $GITHUB_TOKEN is escaped so the helper reads it when git asks, not at build time.
RUN git config --global credential.helper "!f() { echo username=x-access-token; echo password=\$GITHUB_TOKEN; }; f"
ENV TERM=xterm-256color
CMD ["sleep", "infinity"]
