FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends g++ make libcurl4-openssl-dev libminizip-dev zlib1g-dev python3 ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /work
CMD ["sh", "tests/run.sh"]
