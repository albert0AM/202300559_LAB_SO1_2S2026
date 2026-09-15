
FROM alpine:3.20

RUN apk add --no-cache stress-ng

ENV CPU_WORKERS=1
ENV CPU_LOAD=90

CMD stress-ng --cpu ${CPU_WORKERS} --cpu-load ${CPU_LOAD} --timeout 0