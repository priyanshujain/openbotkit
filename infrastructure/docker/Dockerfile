FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /obk .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /obk /usr/local/bin/obk

RUN adduser -D -h /home/obk obk
USER obk
WORKDIR /home/obk

ENTRYPOINT ["obk"]
CMD ["daemon"]
