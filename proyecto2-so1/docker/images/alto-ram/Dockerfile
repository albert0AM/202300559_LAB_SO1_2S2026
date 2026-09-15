FROM alpine:3.20

RUN apk add --no-cache stress-ng

ENV RAM_MB=400

CMD stress-ng --vm 1 --vm-bytes ${RAM_MB}M --vm-keep --timeout 0