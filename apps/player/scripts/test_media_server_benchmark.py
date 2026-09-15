#!/usr/bin/env python3
import importlib.util
import os
import pathlib
import sys
import tempfile
import unittest
from unittest import mock


MODULE_PATH = pathlib.Path(__file__).with_name("media_server_benchmark.py")
SPEC = importlib.util.spec_from_file_location("media_server_benchmark", MODULE_PATH)
assert SPEC and SPEC.loader
benchmark = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = benchmark
SPEC.loader.exec_module(benchmark)


class BenchmarkHelpersTest(unittest.TestCase):
    def test_cookie_header_accepts_http_only_session_cookies(self):
        with tempfile.NamedTemporaryFile("w", encoding="utf-8") as cookies:
            cookies.write("#HttpOnly_127.0.0.1\tTRUE\t/\tTRUE\t0\t__Host-session\tfixture\n")
            cookies.flush()
            self.assertEqual(benchmark.cookie_header(cookies.name), "__Host-session=fixture")

    def test_parse_target_rejects_credentials_and_query_values(self):
        for value in (
            "kinosail=https://user:password@127.0.0.1:38127",
            "kinosail=https://127.0.0.1:38127?token=secret",
            "kinosail=ftp://127.0.0.1:38127",
        ):
            with self.subTest(value=value), self.assertRaises(ValueError):
                benchmark.parse_target(value)

    def test_percentile_interpolates_sorted_samples(self):
        self.assertEqual(benchmark.percentile([9, 1, 5], 0.5), 5)
        self.assertAlmostEqual(benchmark.percentile([1, 2, 3, 4], 0.95), 3.85)

    def test_summarize_keeps_failed_statuses_and_success_count(self):
        summary = benchmark.summarize(
            [
                benchmark.RequestResult(1, 200, 10),
                benchmark.RequestResult(3, 206, 20),
                benchmark.RequestResult(5, 500, 30, "HTTP 500"),
            ]
        )
        self.assertEqual(summary["samples"], 3)
        self.assertEqual(summary["successes"], 2)
        self.assertEqual(summary["statusCodes"], {"200": 1, "206": 1, "500": 1})
        self.assertEqual(summary["bytes"]["total"], 60)
        self.assertEqual(summary["errors"], ["HTTP 500"])

    def test_kind_for_accepts_local_target_names(self):
        self.assertEqual(benchmark.kind_for("jellyfin-local"), "jellyfin")
        self.assertEqual(benchmark.kind_for("plex"), "plex")

    def test_parse_benchmark_dimensions_reject_duplicates_and_unknown_passes(self):
        self.assertEqual(benchmark.parse_int_list("65536,1048576", "range-bytes", 1024, benchmark.MAX_BODY_BYTES), (65536, 1048576))
        with self.assertRaises(ValueError):
            benchmark.parse_int_list("1,1", "concurrency", 1, 64)
        with self.assertRaises(ValueError):
            benchmark.parse_cache_passes("cold,broken")

    def test_kinosail_accepts_a_protected_api_token(self):
        previous = os.environ.get("KINOSAIL_BENCHMARK_TOKEN")
        os.environ["KINOSAIL_BENCHMARK_TOKEN"] = "local-token"
        try:
            adapter = benchmark.Adapter(benchmark.Target("kinosail", "kinosail", "https://127.0.0.1:1", None), True)
            self.assertEqual(adapter.headers, {"Authorization": "Bearer local-token"})
        finally:
            if previous is None:
                os.environ.pop("KINOSAIL_BENCHMARK_TOKEN", None)
            else:
                os.environ["KINOSAIL_BENCHMARK_TOKEN"] = previous

    def test_summarize_reports_time_to_first_byte_and_throughput(self):
        summary = benchmark.summarize(
            [benchmark.RequestResult(10, 200, 100, ttfb_ms=2), benchmark.RequestResult(20, 200, 300, ttfb_ms=3)],
            elapsed_ms=1000,
        )
        self.assertEqual(summary["timeToFirstByteMs"]["p95"], 2.95)
        self.assertEqual(summary["throughput"], {"requestsPerSecond": 2.0, "bytesPerSecond": 400.0})

    def test_resource_delta_parses_podman_stat_units(self):
        before = {"cpu_percent": "1.0%", "mem_percent": "2.0%", "pids": "4", "net_io": "1kB / 2kB", "block_io": "3MB / 4MB"}
        after = {"cpu_percent": "3.5%", "mem_percent": "2.5%", "pids": "6", "net_io": "3kB / 5kB", "block_io": "4MB / 6MB"}
        self.assertEqual(
            benchmark.resource_delta(before, after),
            {"cpuPercent": 2.5, "memPercent": 0.5, "pids": 2.0, "netIOBytes": {"read": 2000, "write": 3000}, "blockIOBytes": {"read": 1000000, "write": 2000000}},
        )

    def test_resource_profile_reports_peak_cpu_memory_and_processes(self):
        profile = benchmark.resource_profile(
            [
                {"cpu_percent": "10%", "mem_percent": "3%", "mem_usage": "100MB / 1GB", "pids": "5", "net_io": "1MB / 2MB"},
                {"cpu_percent": "30%", "mem_percent": "5%", "mem_usage": "200MB / 1GB", "pids": "8", "net_io": "3MB / 5MB"},
            ]
        )
        self.assertEqual(profile["cpuPercent"]["max"], 30.0)
        self.assertEqual(profile["memoryBytes"]["max"], 200000000.0)
        self.assertEqual(profile["pids"]["max"], 8.0)
        self.assertEqual(profile["netIOBytes"], {"read": 2000000, "write": 3000000})

    def test_plex_discovery_includes_playback_and_media_ranges(self):
        adapter = object.__new__(benchmark.Adapter)
        adapter.target = benchmark.Target("plex", "plex", "http://127.0.0.1:1", None)
        adapter.item = {}
        responses = iter(
            [
                (benchmark.RequestResult(1, 200, 0), b'<MediaContainer><Directory type="movie" key="1" /></MediaContainer>'),
                (
                    benchmark.RequestResult(1, 200, 0),
                    b'<MediaContainer><Video ratingKey="42"><Media><Part key="/library/parts/7/file" /></Media></Video></MediaContainer>',
                ),
            ]
        )
        adapter.request = lambda _spec: next(responses)
        specs = adapter._discover_plex((65536, 1048576, 4194304))
        self.assertEqual(specs["playback_plan"].path, "/library/metadata/42?checkFiles=1")
        self.assertEqual(specs["media_range_64k"].path, "/library/parts/7/file")
        self.assertEqual(specs["media_range_64k"].headers, (("Range", "bytes=0-65535"),))

    def test_plex_discovery_rejects_nonlocal_media_part_url(self):
        adapter = object.__new__(benchmark.Adapter)
        adapter.target = benchmark.Target("plex", "plex", "http://127.0.0.1:1", None)
        adapter.item = {}
        responses = iter(
            [
                (benchmark.RequestResult(1, 200, 0), b'<MediaContainer><Directory type="movie" key="1" /></MediaContainer>'),
                (
                    benchmark.RequestResult(1, 200, 0),
                    b'<MediaContainer><Video ratingKey="42"><Media><Part key="https://attacker.invalid/file" /></Media></Video></MediaContainer>',
                ),
            ]
        )
        adapter.request = lambda _spec: next(responses)
        with self.assertRaises(ValueError):
            adapter._discover_plex((65536,))

    def test_plex_requests_default_to_xml(self):
        adapter = object.__new__(benchmark.Adapter)
        adapter.target = benchmark.Target("plex", "plex", "http://127.0.0.1:1", None)
        adapter.context = None
        adapter.headers = {"X-Plex-Token": "local-token"}
        with mock.patch.object(benchmark.urllib.request, "urlopen", side_effect=OSError):
            adapter.request(benchmark.RequestSpec("/identity"))
            request = benchmark.urllib.request.urlopen.call_args.args[0]
            self.assertEqual(request.headers["Accept"], "application/xml")


if __name__ == "__main__":
    unittest.main()
