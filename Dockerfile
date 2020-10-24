FROM golang:1.15.3-alpine AS build
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -o /acm-secrets-controller .
CMD /acm-secrets-controller
