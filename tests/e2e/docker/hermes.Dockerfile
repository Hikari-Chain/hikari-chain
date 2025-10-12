ARG PLATFORM=linux/amd64
FROM --platform=$PLATFORM informalsystems/hermes:1.10.0 AS hermes-builder

FROM --platform=$PLATFORM debian:buster-slim
USER root

COPY --from=hermes-builder /usr/bin/hermes /usr/local/bin/
RUN chmod +x /usr/local/bin/hermes

EXPOSE 3031