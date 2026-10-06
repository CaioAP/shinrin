# One image for every role: `shinrin api`, `shinrin worker`, `shinrin migrate`
# and one-off `shinrin run <routine>`. Builds natively for amd64 and arm64
# (Oracle's free Ampere servers are arm64).
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/shinrin ./cmd/shinrin

# distroless/static has CA certificates and no shell; the binary embeds its
# migrations and the time zone database.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/shinrin /shinrin
ENV SHINRIN_ADDR=:8080
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/shinrin"]
CMD ["api"]
