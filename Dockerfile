FROM golang:1.18-alpine AS build

WORKDIR /src/
COPY main.go go.* /src/
RUN CGO_ENABLED=0 go build -o /bin/argocd-secure-app-plugin

FROM scratch
COPY --from=build /bin/argocd-secure-app-plugin /bin/argocd-secure-app-plugin
ENTRYPOINT ["/bin/argocd-secure-app-plugin"]
