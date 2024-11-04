FROM rust:1.78.0-buster as builder
WORKDIR /usr/src/test-rust-logger
COPY . .
RUN cargo install --path .

FROM debian:bullseye-slim
COPY --from=builder /usr/local/cargo/bin/test-rust-logger /usr/local/bin/test-rust-logger
CMD ["test-rust-logger"]
