FROM mcr.microsoft.com/devcontainers/base:ubuntu

RUN --mount=type=cache,target=/var/cache/apt \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update && \
    apt-get install -y vim wget curl
 
ENV PATH=/usr/local/go/bin:$PATH
CMD ["/bin/bash"]
