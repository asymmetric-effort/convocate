# Ubuntu base image for all Convocate build stages
# Pre-installs common packages to avoid repeated apt-get update/install

FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl && \
    rm -rf /var/lib/apt/lists/*
