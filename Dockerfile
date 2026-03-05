FROM ubuntu:22.04

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    python3 \
    python3-pip \
    make \
    gcc \
    g++ \
    build-essential \
    ca-certificates \
    gnupg \
    lsb-release \
    sudo \
    vim \
    && rm -rf /var/lib/apt/lists/*

# Install latest Node.js via NodeSource repository (current version)
RUN curl -fsSL https://deb.nodesource.com/setup_current.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install PNPM package manager (latest version)
RUN npm install -g pnpm@latest

# Install AI coding CLIs
RUN npm install -g @openai/codex @google/gemini-cli

# Install latest Go
RUN curl -fsSL https://go.dev/dl/go1.23.5.linux-amd64.tar.gz -o go.tar.gz \
    && tar -C /usr/local -xzf go.tar.gz \
    && rm go.tar.gz

# Install Docker CLI (for docker-in-docker scenarios)
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null \
    && apt-get update && apt-get install -y docker-ce-cli && rm -rf /var/lib/apt/lists/*

# Install Playwright browser + deps into a shared path for test runs
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
RUN npx -y playwright@latest install --with-deps chromium \
    && chmod -R a+rX /ms-playwright

# Create a user for development
RUN useradd -m -s /bin/bash developer && \
    echo 'developer ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Set working directory
WORKDIR /workspace

# Switch to developer user
USER developer

# Install Claude Code CLI as the developer user with proper HOME
ENV HOME=/home/developer
RUN curl -fsSL https://claude.ai/install.sh | HOME=/home/developer bash

# Install uv for Python package management as developer user
RUN curl -LsSf https://astral.sh/uv/install.sh | HOME=/home/developer sh

# Set up environment variables for PATH
RUN echo 'export PATH="/home/developer/.local/bin:/usr/local/go/bin:$PATH"' >> /home/developer/.bashrc && \
    echo 'export GOPATH=/home/developer/go' >> /home/developer/.bashrc && \
    echo 'export PATH="$GOPATH/bin:$PATH"' >> /home/developer/.bashrc

# Ensure the PATH is set for interactive shells
RUN echo 'if [ -f ~/.bashrc ]; then source ~/.bashrc; fi' >> /home/developer/.bash_profile

# Ensure Claude CLI is in PATH for this build
ENV PATH="/home/developer/.local/bin:/usr/local/go/bin:$PATH"
ENV GOPATH="/home/developer/go"

# Copy entrypoint script
COPY --chown=developer:developer entrypoint.sh /home/developer/entrypoint.sh
RUN chmod +x /home/developer/entrypoint.sh

# Use entrypoint script that allows passing additional flags like --resume
ENTRYPOINT ["/home/developer/entrypoint.sh"]
