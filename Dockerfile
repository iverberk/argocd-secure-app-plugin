FROM golang:1.20.1 as builder

ARG YQ_VERSION=4.30.8 #https://github.com/mikefarah/yq/releases
ARG KUSTOMIZE_VERSION=5.0.0 #https://github.com/kubernetes-sigs/kustomize/releases
ARG HELM_VERSION=3.11.1 #https://github.com/helm/helm/releases
ARG KSOPS_VERSION=4.1.0 #https://githbu.com/viaduct-ai/kustomize-sops/releases

RUN apt update && apt install -y curl wget unzip git golint && rm -rf /var/lib/apt/lists/*

# Install Helm
RUN curl -SL https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz | tar -xz linux-amd64/helm && mv linux-amd64/helm /usr/local/bin/

# Install Kustomize
RUN curl -SL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz | tar -xzC /usr/local/bin

# Install yq
RUN curl -L -s "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" -o /usr/local/bin/yq && chmod +x /usr/local/bin/yq

# Install ksops
RUN curl -L -s "https://github.com/viaduct-ai/kustomize-sops/releases/download/v${KSOPS_VERSION}/ksops_${KSOPS_VERSION}_Linux_x86_64.tar.gz" | tar -xzC /usr/local/bin

WORKDIR /build
ADD . /build
RUN CGO_ENABLED=0 go build -o /bin/argocd-secure-app-plugin

FROM alpine:3.17.2

COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/kustomize
COPY --from=builder /usr/local/bin/ksops /usr/local/bin/ksops
COPY --from=builder /bin/argocd-secure-app-plugin /usr/local/bin/argocd-secure-app-plugin
COPY ./plugin.yaml /home/argocd/cmp-server/config/plugin.yaml

USER 999

# This does NOT exist inside the image, must be mounted from argocd
ENTRYPOINT [ "/var/run/argocd/argocd-cmp-server" ]
