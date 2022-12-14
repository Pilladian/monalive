## Build
FROM golang:1.16-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /monalive

## Deploy
FROM gcr.io/distroless/base-debian10

ENV TZ="Europe/Berlin"

ENV BOT_TOKEN="YOUR_BOT_TOKEN_GOES_HERE"
ENV CHAT_ID="YOUR_CHAT_ID_GOES_HERE"
ENV EXTERNAL_PROXY_URL="YOUR_EXTERNAL_PROXY_URL_GOES_HERE"
ENV EXTERNAL_PROXY_HOST="YOUR_EXTERNAL_PROXY_HOST_GOES_HERE"
ENV INTERNAL_PROXY_URL="YOUR_INTERNAL_PROXY_URL_GOES_HERE"
ENV INTERNAL_PROXY_HOST="YOUR_INTERNAL_PROXY_HOST_GOES_HERE"

ENV URL_1="YOUR_FIRST_MON_URL_GOES_HERE"
ENV URL_2="YOUR_SECOND_MON_URL_GOES_HERE"
ENV URL_3="YOUR_THIRD_MON_URL_GOES_HERE"

ENV INFOWATCH_URL="YOUR_INFOWATCH_API_URL_GOES_HERE"
ENV INFOWATCH_REV_PROXY_USERNAME="YOUR_INFOWATCH_REV_PROXY_USERNAME_GOES_HERE"
ENV INFOWATCH_REV_PROXY_PASSWORD="YOUR_INFOWATCH_INFOWATCH_REV_PROXY_PASSWORD_GOES_HERE"
ENV INFOWATCH_PID="YOUR_INFOWATCH_PID_GOES_HERE"

ENV ELASTIC_SEARCH_URL="YOUR_ELASTIC_SEARCH_URL_GOES_HERE"
ENV ELASTIC_SEARCH_USERNAME="YOUR_ELASTIC_SEARCH_USERNAME_GOES_HERE"
ENV ELASTIC_SEARCH_PASSWORD="YOUR_INFOWATCH_ELASTIC_SEARCH_PASSWORD_GOES_HERE"
ENV ELASTIC_SEARCH_PID="YOUR_ELASTIC_SEARCH_PID_GOES_HERE"

WORKDIR /tmp/

COPY --from=build /monalive /monalive

USER nonroot:nonroot

ENTRYPOINT ["/monalive"]