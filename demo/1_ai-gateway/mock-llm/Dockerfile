# Build a tiny static Go binary for the mock OpenAI-compatible LLM server.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /mock-llm .

FROM scratch
COPY --from=build /mock-llm /mock-llm
EXPOSE 8080
ENTRYPOINT ["/mock-llm"]
