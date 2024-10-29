FROM rust:1.75-slim as builder

WORKDIR /usr/src/app
COPY . .
RUN cargo build --release

FROM debian:bookworm-slim
COPY --from=builder /usr/src/app/target/release/test-rust-logger /usr/local/bin/

CMD ["test-rust-logger"]
