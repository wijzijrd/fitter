FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/fitter-svc .

FROM alpine:3.24
# Outbound TLS to Strava and to Postgres needs a trust store; the scratch alpine base has none.
RUN apk add --no-cache ca-certificates
COPY --from=build /out/fitter-svc /usr/local/bin/fitter-svc
ENV PORT=8081
EXPOSE 8081
CMD ["fitter-svc"]
