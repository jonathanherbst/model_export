FROM mcr.microsoft.com/devcontainers/base:noble
ARG nodjs_version=v24.14.1
ARG go_version=1.26.2

RUN apt update && apt install -y cmake libbz2-dev

# install golang
RUN wget https://go.dev/dl/go$go_version.linux-amd64.tar.gz && tar -C /usr/local -xzf go$go_version.linux-amd64.tar.gz
ENV PATH="$PATH:/usr/local/go/bin"

# install nodejs
RUN wget https://nodejs.org/dist/$nodjs_version/node-$nodjs_version-linux-x64.tar.gz && tar -C /usr/local -xzf node-$nodjs_version-linux-x64.tar.gz
ENV PATH="$PATH:/usr/local/node-$nodjs_version-linux-x64/bin/"