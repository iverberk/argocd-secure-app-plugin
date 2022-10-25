FROM golang:1.18-alpine AS build

WORKDIR /src/
COPY *.go go.* /src/
RUN CGO_ENABLED=0 go build -o /bin/argocd-secure-app-plugin

FROM alpine

COPY --from=build /bin/argocd-secure-app-plugin .
USER 999

ENTRYPOINT [ "cp", "argocd-secure-app-plugin", "/custom-tools/" ]
