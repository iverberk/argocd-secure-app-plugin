FROM golang:1.18-alpine AS build

WORKDIR /src/
COPY main.go go.* /src/
RUN CGO_ENABLED=0 go build -o /bin/argocd-secure-app-plugin

FROM alpine as putter
COPY --from=build /bin/argocd-secure-app-plugin .
USER 999
ENTRYPOINT [ "cp", "argocd-secure-app-plugin", "/custom-tools/" ]
