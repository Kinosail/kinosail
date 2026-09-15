#!/usr/bin/env python3
"""Compare common media-server request and media-read workloads."""

from __future__ import annotations

import argparse
import json
import sys

try:
    from scripts.media_server_benchmark_core import *
    from scripts.media_server_benchmark_runtime import *
except ModuleNotFoundError:
    from media_server_benchmark_core import *
    from media_server_benchmark_runtime import *

def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", action="append", required=True, help="name=base URL; repeat for Kinosail, Jellyfin, and Plex")
    parser.add_argument("--container", action="append", default=[], help="name=Podman container for resource samples")
    parser.add_argument("--iterations", type=int, default=5, help="concurrent rounds per workload")
    parser.add_argument("--concurrency", default="1,4,8", help="comma-separated concurrent request counts")
    parser.add_argument("--warmup", type=int, default=1, help="warmup rounds per workload")
    parser.add_argument("--duration", type=int, default=0, help="sustained workload duration in seconds; 0 uses burst iterations")
    parser.add_argument("--cache-passes", default="warm", help="comma-separated cache passes: cold, warm")
    parser.add_argument("--range-bytes", default="65536,1048576,4194304", help="comma-separated media range sizes")
    parser.add_argument("--startup-restarts", type=int, default=0, help="restart each benchmark container this many times and measure readiness")
    parser.add_argument("--output", default="-", help="JSON output path, or - for stdout")
    parser.add_argument("--insecure", action="store_true", help="allow local self-signed HTTPS certificates")
    return parser


def validate_args(args: argparse.Namespace) -> tuple[list[Target], dict[str, str], list[int], tuple[int, ...], tuple[str, ...]]:
    if not 1 <= args.iterations <= 1000 or not 0 <= args.warmup <= 100:
        raise ValueError("iterations must be 1..1000 and warmup must be 0..100")
    if not 0 <= args.duration <= 300 or not 0 <= args.startup_restarts <= 5:
        raise ValueError("duration must be 0..300 seconds and startup-restarts must be 0..5")
    concurrency = parse_int_list(args.concurrency, "concurrency", 1, 64)
    range_sizes = parse_int_list(args.range_bytes, "range-bytes", 1024, MAX_BODY_BYTES)
    cache_passes = parse_cache_passes(args.cache_passes)
    targets: list[Target] = []
    seen: set[str] = set()
    for value in args.server:
        name, base_url = parse_target(value)
        if name in seen:
            raise ValueError(f"duplicate server: {name}")
        seen.add(name)
        targets.append(Target(name, kind_for(name), base_url, None))
    containers = dict(parse_mapping(value, "container") for value in args.container)
    return [Target(target.name, target.kind, target.base_url, containers.get(target.name)) for target in targets], containers, list(concurrency), range_sizes, cache_passes


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        targets, _, concurrency, range_sizes, cache_passes = validate_args(args)
    except ValueError as error:
        print(f"error: {error}", file=sys.stderr)
        return 2
    output: dict[str, Any] = {
        "schemaVersion": 1,
        "generatedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "host": host_info(),
        "parameters": {"iterations": args.iterations, "warmup": args.warmup, "durationSeconds": args.duration, "cachePasses": cache_passes, "rangeBytes": range_sizes, "startupRestarts": args.startup_restarts, "concurrency": concurrency},
        "targets": [],
    }
    ready = 0
    for target in targets:
        record: dict[str, Any] = {"name": target.name, "kind": target.kind, "baseUrl": target.base_url, "container": container_info(target.container), "containerFootprint": container_footprint(target.container), "workloads": []}
        if not os.environ.get(f"{target.kind.upper()}_BENCHMARK_TOKEN", "") and not (target.kind == "kinosail" and os.environ.get("KINOSAIL_BENCHMARK_COOKIE_FILE", "")):
            record.update({"status": "skipped", "reason": f"missing {target.kind} benchmark authentication"})
            output["targets"].append(record)
            print(f"{target.name}: skipped (missing authentication)", file=sys.stderr)
            continue
        try:
            adapter = Adapter(target, args.insecure)
            specs = adapter.discover(range_sizes)
            record["serverVersion"] = adapter.server_version()
        except (OSError, ValueError) as error:
            record.update({"status": "error", "reason": str(error)})
            output["targets"].append(record)
            print(f"{target.name}: error ({error})", file=sys.stderr)
            continue
        ready += 1
        record["status"] = "ready"
        record["imageDigest"] = (record["container"] or {}).get("imageDigest")
        if args.startup_restarts and target.container:
            record["startup"] = [restart_and_measure(adapter, target.container, specs["library"]) for _ in range(args.startup_restarts)]
        for workload, spec in specs.items():
            for cache_pass in cache_passes:
                if cache_pass == "warm":
                    for _ in range(args.warmup):
                        adapter.request(spec)
                for level in concurrency:
                    before = container_info(target.container)
                    summary = run_workload(adapter, spec, args.iterations, level, args.duration, target.container)
                    after = container_info(target.container)
                    record["workloads"].append({"name": workload, "cache": cache_pass, "concurrency": level, "before": before, "after": after, "resourceDelta": resource_delta(before, after), **summary})
                    print(f"{target.name} {workload} {cache_pass} c={level}: p95={summary['latencyMs']['p95']:.1f}ms rps={summary.get('throughput', {}).get('requestsPerSecond', 0):.1f} successes={summary['successes']}/{summary['samples']}", file=sys.stderr)
        output["targets"].append(record)
    rendered = json.dumps(output, indent=2, sort_keys=True) + "\n"
    if args.output == "-":
        sys.stdout.write(rendered)
    else:
        with open(args.output, "w", encoding="utf-8") as destination:
            destination.write(rendered)
        print(f"results: {args.output}", file=sys.stderr)
    return 0 if ready else 1


if __name__ == "__main__":
    raise SystemExit(main())
