"""Run benchmark workloads and collect container resource data."""

from __future__ import annotations

import argparse
import concurrent.futures
import json
import os
import platform
import subprocess
import threading
import time
from statistics import mean
from typing import Any

try:
    from scripts.media_server_benchmark_core import *
except ModuleNotFoundError:
    from media_server_benchmark_core import *

def podman(command: list[str]) -> str:
    try:
        completed = subprocess.run(["podman", *command], capture_output=True, text=True, timeout=10, check=False)
    except (OSError, subprocess.TimeoutExpired):
        return ""
    return completed.stdout.strip() if completed.returncode == 0 else ""


def podman_success(command: list[str]) -> bool:
    try:
        completed = subprocess.run(["podman", *command], capture_output=True, text=True, timeout=30, check=False)
    except (OSError, subprocess.TimeoutExpired):
        return False
    return completed.returncode == 0


def container_info(name: str | None) -> dict[str, Any] | None:
    if not name:
        return None
    digest = podman(["inspect", "--format", "{{.ImageDigest}}", name])
    raw = podman(["stats", "--no-stream", "--format", "json", name])
    if not raw:
        return {"name": name, "imageDigest": digest or None}
    try:
        rows = json.loads(raw)
        row = rows[0] if rows else {}
    except (json.JSONDecodeError, IndexError, TypeError):
        row = {}
    result: dict[str, Any] = {"name": name, "imageDigest": digest or None}
    for key in ("cpu_time", "avg_cpu", "cpu_percent", "mem_usage", "mem_percent", "pids", "net_io", "block_io"):
        if key in row:
            result[key] = row[key]
    return result


def container_stats(name: str | None) -> dict[str, Any] | None:
    if not name:
        return None
    raw = podman(["stats", "--no-stream", "--format", "json", name])
    if not raw:
        return None
    try:
        rows = json.loads(raw)
        row = rows[0] if rows else {}
    except (json.JSONDecodeError, IndexError, TypeError):
        return None
    return {key: row[key] for key in ("cpu_time", "avg_cpu", "cpu_percent", "mem_usage", "mem_percent", "pids", "net_io", "block_io") if key in row}


def container_footprint(name: str | None) -> dict[str, Any] | None:
    if not name:
        return None
    raw = podman(["inspect", "--size", "--format", "{{.Image}}|{{.SizeRootFs}}|{{.SizeRw}}|{{.HostConfig.Memory}}|{{.HostConfig.NanoCpus}}|{{.Config.Image}}", name])
    if not raw:
        return None
    image_id, root_fs, writable, memory_limit, nano_cpus, image_name = (raw.split("|", 5) + [""] * 6)[:6]
    footprint: dict[str, Any] = {"image": image_name or None, "imageID": image_id or None}
    for value, key in ((root_fs, "rootFsBytes"), (writable, "writableBytes"), (memory_limit, "memoryLimitBytes"), (nano_cpus, "nanoCpus")):
        if value.isdigit():
            footprint[key] = int(value)
    image_size = podman(["image", "inspect", "--format", "{{.Size}}", image_id]) if image_id else ""
    if image_size.isdigit():
        footprint["imageBytes"] = int(image_size)
    return footprint


def stat_number(value: Any) -> float | None:
    match = re.fullmatch(r"\s*([0-9]+(?:\.[0-9]+)?)\s*%?\s*", str(value))
    return float(match.group(1)) if match else None


def io_bytes(value: Any) -> tuple[float, float] | None:
    matches = re.findall(r"([0-9]+(?:\.[0-9]+)?)\s*([kmgt]?i?b)", str(value).lower())
    if len(matches) < 2:
        return None
    units = {"b": 1, "kb": 1000, "kib": 1024, "mb": 1000**2, "mib": 1024**2, "gb": 1000**3, "gib": 1024**3, "tb": 1000**4, "tib": 1024**4}
    return tuple(float(number) * units[unit] for number, unit in matches[:2])  # type: ignore[return-value]


def memory_bytes(value: Any) -> float | None:
    parsed = io_bytes(value)
    return parsed[0] if parsed else None


def resource_profile(samples: list[dict[str, Any]]) -> dict[str, Any]:
    if not samples:
        return {}
    profile: dict[str, Any] = {"samples": len(samples)}
    for source, output in (("cpu_percent", "cpuPercent"), ("mem_percent", "memoryPercent"), ("pids", "pids"), ("mem_usage", "memoryBytes")):
        values = [parsed for sample in samples if (parsed := memory_bytes(sample.get(source)) if source == "mem_usage" else stat_number(sample.get(source))) is not None]
        if values:
            profile[output] = {"p50": round(percentile(values, 0.50), 3), "p95": round(percentile(values, 0.95), 3), "max": round(max(values), 3)}
    for source, output in (("net_io", "netIOBytes"), ("block_io", "blockIOBytes")):
        values = [parsed for sample in samples if (parsed := io_bytes(sample.get(source))) is not None]
        if len(values) >= 2:
            profile[output] = {"read": round(values[-1][0] - values[0][0]), "write": round(values[-1][1] - values[0][1])}
    return profile


def resource_delta(before: dict[str, Any] | None, after: dict[str, Any] | None) -> dict[str, Any]:
    if not before or not after:
        return {}
    delta: dict[str, Any] = {}
    for source, output in (("cpu_percent", "cpuPercent"), ("mem_percent", "memPercent"), ("pids", "pids")):
        left, right = stat_number(before.get(source)), stat_number(after.get(source))
        if left is not None and right is not None:
            delta[output] = round(right - left, 3)
    for source, output in (("net_io", "netIOBytes"), ("block_io", "blockIOBytes")):
        left, right = io_bytes(before.get(source)), io_bytes(after.get(source))
        if left and right:
            delta[output] = {"read": round(right[0] - left[0]), "write": round(right[1] - left[1])}
    return delta


def summarize(results: list[RequestResult], elapsed_ms: float | None = None) -> dict[str, Any]:
    latencies = [result.latency_ms for result in results]
    ttfb = [result.ttfb_ms for result in results if result.ttfb_ms]
    successes = [result for result in results if 200 <= result.status < 300]
    errors = sorted({result.error for result in results if result.error})[:3]
    summary: dict[str, Any] = {
        "samples": len(results),
        "successes": len(successes),
        "statusCodes": {str(code): sum(result.status == code for result in results) for code in sorted({result.status for result in results})},
        "latencyMs": {
            "min": round(min(latencies), 3) if latencies else 0,
            "mean": round(mean(latencies), 3) if latencies else 0,
            "p50": round(percentile(latencies, 0.50), 3),
            "p95": round(percentile(latencies, 0.95), 3),
            "p99": round(percentile(latencies, 0.99), 3),
            "max": round(max(latencies), 3) if latencies else 0,
        },
        "bytes": {"total": sum(result.body_bytes for result in results), "mean": round(mean([result.body_bytes for result in results]), 1) if results else 0},
        **({"errors": errors} if errors else {}),
    }
    if ttfb:
        summary["timeToFirstByteMs"] = {
            "p50": round(percentile(ttfb, 0.50), 3),
            "p95": round(percentile(ttfb, 0.95), 3),
            "p99": round(percentile(ttfb, 0.99), 3),
        }
    if elapsed_ms and elapsed_ms > 0:
        summary["elapsedMs"] = round(elapsed_ms, 3)
        summary["throughput"] = {
            "requestsPerSecond": round(len(results) / (elapsed_ms / 1000), 3),
            "bytesPerSecond": round(sum(result.body_bytes for result in results) / (elapsed_ms / 1000), 3),
        }
    return summary


def sustained_worker(adapter: Adapter, spec: RequestSpec, deadline: float) -> list[RequestResult]:
    results: list[RequestResult] = []
    while time.monotonic() < deadline:
        results.append(adapter.request(spec)[0])
    return results


def run_workload(adapter: Adapter, spec: RequestSpec, rounds: int, concurrency: int, duration_s: int = 0, container: str | None = None) -> dict[str, Any]:
    resource_samples: list[dict[str, Any]] = []
    stop_sampling = threading.Event()
    sampler: threading.Thread | None = None
    if container and duration_s:
        def sample_resources() -> None:
            while not stop_sampling.is_set():
                sample = container_stats(container)
                if sample:
                    resource_samples.append(sample)
                stop_sampling.wait(0.25)

        sampler = threading.Thread(target=sample_resources, daemon=True)
        sampler.start()
    started = time.perf_counter()
    try:
        if duration_s:
            deadline = time.monotonic() + duration_s
            with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
                futures = [executor.submit(sustained_worker, adapter, spec, deadline) for _ in range(concurrency)]
                results = [result for future in futures for result in future.result()]
        else:
            results = []
            for _ in range(rounds):
                with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
                    futures = [executor.submit(adapter.request, spec) for _ in range(concurrency)]
                    results.extend(future.result()[0] for future in futures)
    finally:
        stop_sampling.set()
        if sampler:
            sampler.join(timeout=2)
    summary = summarize(results, (time.perf_counter() - started) * 1000)
    if resource_samples:
        summary["resourceProfile"] = resource_profile(resource_samples)
    return summary


def wait_for_ready(adapter: Adapter, readiness: RequestSpec, timeout_s: int = 120) -> float | None:
    started = time.perf_counter()
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline:
        result, _ = adapter.request(readiness)
        if 200 <= result.status < 300:
            return (time.perf_counter() - started) * 1000
        time.sleep(0.25)
    return None


def restart_and_measure(adapter: Adapter, container: str, readiness: RequestSpec) -> dict[str, Any]:
    started = time.perf_counter()
    if not podman_success(["restart", container]):
        return {"success": False, "error": "podman restart failed"}
    ready_ms = wait_for_ready(adapter, readiness)
    return {"success": ready_ms is not None, "readyMs": round((ready_ms or (time.perf_counter() - started) * 1000), 3), **({"error": "health check timed out"} if ready_ms is None else {})}


def parse_int_list(value: str, label: str, minimum: int, maximum: int) -> tuple[int, ...]:
    try:
        values = tuple(int(part) for part in value.split(","))
    except ValueError as error:
        raise ValueError(f"{label} must contain integers") from error
    if not values or len(set(values)) != len(values) or any(value < minimum or value > maximum for value in values):
        raise ValueError(f"{label} values must be {minimum}..{maximum} without duplicates")
    return values


def parse_cache_passes(value: str) -> tuple[str, ...]:
    passes = tuple(part.strip() for part in value.split(","))
    if not passes or len(set(passes)) != len(passes) or any(value not in {"cold", "warm"} for value in passes):
        raise ValueError("cache-passes must contain cold and/or warm without duplicates")
    return passes


def host_info() -> dict[str, Any]:
    info: dict[str, Any] = {"os": platform.system(), "arch": platform.machine(), "cpuCount": os.cpu_count()}
    try:
        info["memoryBytes"] = os.sysconf("SC_PHYS_PAGES") * os.sysconf("SC_PAGE_SIZE")
    except (AttributeError, OSError, ValueError):
        pass
    return info
