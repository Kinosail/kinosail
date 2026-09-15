# Media server benchmark

This benchmark compares Kinosail, Jellyfin, and Plex on the same host and media fixture.
It measures request latency, time to first byte, concurrent request behavior, sustained throughput, media range reads, playback-plan latency, startup readiness, response bytes, container image and writable-layer size, memory use, CPU use, process count, network I/O, block I/O, and Podman resource samples.

The benchmark does not combine direct playback and transcoding into one score.
Run those modes as separate experiments with the same source file, client profile, and hardware acceleration setting.

## Local setup

Use a separate Podman container and data volume for each server.
Mount the same read-only media directory into every container.
Use a Plex claim token and a Plex benchmark token when Plex is included.

The Kinosail fixture can be generated with:

```sh
CONTAINER_ENGINE=podman KINOSAIL_TEST_IMAGE=localhost/kinosail:test-instance \
  ./scripts/generate-test-media.sh /absolute/path/to/benchmark-media
```

Initialize Jellyfin with one benchmark user and a movie library at `/media/Movies`.
Initialize Plex with one benchmark user and a movie library at `/media/Movies`.
Keep the credentials in environment variables or protected files.
Never put tokens in this document or in result files.

## Run

Create a Kinosail cookie file by logging into a local test instance, or set a protected `KINOSAIL_BENCHMARK_TOKEN` for a local API session token.
Set the Jellyfin user ID and token from the same local setup.
Set `PLEX_BENCHMARK_TOKEN` only when Plex setup is complete.

```sh
export KINOSAIL_BENCHMARK_COOKIE_FILE=/protected/kinosail-cookies
export JELLYFIN_BENCHMARK_TOKEN=protected-token
export JELLYFIN_BENCHMARK_USER_ID=protected-user-id
export PLEX_BENCHMARK_TOKEN=protected-token

./scripts/benchmark-media-servers.sh \
  --server kinosail=https://127.0.0.1:38127 \
  --server jellyfin=http://127.0.0.1:18096 \
  --server plex=http://127.0.0.1:13240 \
  --container kinosail=kinosail-benchmark \
  --container jellyfin=jellyfin-benchmark \
  --container plex=plex-benchmark \
  --insecure \
  --iterations 5 \
  --concurrency 1,4,8 \
  --output /tmp/media-server-benchmark.json
```

Each concurrency value is one burst per iteration by default.
Use `--duration` for sustained load. Each worker sends requests until the duration ends.
Use `--cache-passes cold,warm` to record the first post-discovery pass and a warmed pass separately.
Use `--startup-restarts 2` to restart each supplied benchmark container and measure health readiness.
The result contains p50, p95, p99, time to first byte, request and byte throughput, success counts, status codes, response bytes, image digests, image/rootfs/writable-layer sizes, configured CPU and memory limits, startup readiness, and before/after Podman samples with deltas.
Sustained workloads also include sampled CPU-percent, memory-percent, memory-bytes, process-count, network-I/O, and block-I/O profiles.

A broader local run is:

```sh
./scripts/benchmark-media-servers.sh \
  --server kinosail=https://127.0.0.1:18128 \
  --server jellyfin=http://127.0.0.1:18096 \
  --server plex=http://127.0.0.1:13240 \
  --container kinosail=kinosail-bench-kinosail \
  --container jellyfin=kinosail-bench-jellyfin \
  --container plex=kinosail-bench-plex \
  --insecure --iterations 3 --duration 5 --concurrency 1,4,8,16 \
  --cache-passes cold,warm --startup-restarts 1 \
  --range-bytes 65536,1048576,4194304 \
  --output /tmp/media-server-benchmark-comprehensive.json
```

The range workloads measure direct media reads at three request sizes.
Kinosail, Jellyfin, and Plex expose a playback-plan workload when their authenticated discovery succeeds.
Plex is skipped unless its local test server is claimed and has a protected benchmark token.

Plex is recorded as skipped when its token is absent.
This keeps an unavailable comparison visible without fabricating a result.

## Comparison rules

- Pin image digests and record the server version.
- Use the same CPU and memory limits for every container.
- Compare both absolute memory and memory as a percentage of the configured limit.
- Treat CPU percent as a container measurement, not a host-wide measurement.
- Use the same media files and storage type.
- Run cold-cache and warm-cache passes separately.
- Keep direct play, audio transcode, and video transcode separate.
- Treat playback-plan results as control-plane measurements, not transcoding proof.
- Repeat each case and compare p95 and p99, not only averages.
- Record dropped frames, rebuffering, and time to the second moving frame in browser playback tests.
- Report hardware-transcoding support as a separate matrix dimension.
