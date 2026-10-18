FROM rust:1-slim-bookworm AS build
WORKDIR /app
COPY Cargo.toml ./
COPY src ./src
RUN cargo build --release

FROM debian:bookworm-slim
COPY --from=build /app/target/release/rust-api /usr/local/bin/rust-api
EXPOSE 8000
CMD ["rust-api"]
