#Start temp envrioment with go installed & set workign image directory
FROM golang:1.27.1-alpine AS builder
WORKDIR /src

#copy source code and go moduel
COPY go.mod ./
COPY tracker/ ./tracker/

#Let docker select OS and architecture
ARG TARGETOS
ARG TARGETARCH

#Complie tracker
RUN mkdir -p /out && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/tracker ./tracker

FROM alpine:3.22

#Create non-rootuser and only comply executable from the builder
RUN addgroup -S tracker && adduser -S tracker -G tracker
COPY --from=builder /out/tracker /tracker

#Run as non-root user and expose port 8080 for tracker
USER tracker
EXPOSE 8080

#Starts tracker when container is initalised
ENTRYPOINT [ "/tracker" ]