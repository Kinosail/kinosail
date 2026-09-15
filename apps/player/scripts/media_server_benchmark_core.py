#!/usr/bin/env python3
"""Compare common media-server request and media-read workloads."""

from __future__ import annotations

import argparse
import concurrent.futures
import datetime as dt
import json
import os
import platform
import re
import ssl
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from statistics import mean
from typing import Any


MAX_BODY_BYTES = 4 * 1024 * 1024
SERVER_NAME = re.compile(r"^[a-z][a-z0-9_-]{1,31}$")


@dataclass(frozen=True)
class Target:
    name: str
    kind: str
    base_url: str
    container: str | None


@dataclass(frozen=True)
class RequestSpec:
    path: str
    headers: tuple[tuple[str, str], ...] = ()


@dataclass(frozen=True)
class RequestResult:
    latency_ms: float
    status: int
    body_bytes: int
    error: str = ""
    ttfb_ms: float = 0


def parse_target(value: str) -> tuple[str, str]:
    name, separator, base_url = value.partition("=")
    if not separator or not SERVER_NAME.fullmatch(name):
        raise ValueError("server must use a lowercase name and URL, for example kinosail=https://127.0.0.1:38127")
    parsed = urllib.parse.urlsplit(base_url)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError(f"invalid server URL for {name}")
    if not any(part in name.lower() for part in ("kinosail", "jellyfin", "plex")):
        raise ValueError(f"server name must contain kinosail, jellyfin, or plex: {name}")
    return name, base_url.rstrip("/")


def parse_mapping(value: str, label: str) -> tuple[str, str]:
    name, separator, mapped = value.partition("=")
    if not separator or not SERVER_NAME.fullmatch(name) or not mapped or "/" in mapped:
        raise ValueError(f"{label} must use name=value")
    return name, mapped


def kind_for(name: str) -> str:
    lower = name.lower()
    for kind in ("kinosail", "jellyfin", "plex"):
        if kind in lower:
            return kind
    raise ValueError(f"unsupported server kind: {name}")


def cookie_header(path: str) -> str:
    if os.path.getsize(path) > 1 * 1024 * 1024:
        raise ValueError("cookie file is too large")
    values: list[str] = []
    with open(path, encoding="utf-8") as cookies:
        for line in cookies:
            if line.startswith("#HttpOnly_"):
                line = line[len("#HttpOnly_") :]
            elif not line.strip() or line.startswith("#"):
                continue
            fields = line.rstrip("\n").split("\t")
            if len(fields) >= 7 and fields[5] and fields[6]:
                values.append(f"{fields[5]}={fields[6]}")
    if not values:
        raise ValueError(f"cookie file has no cookies: {path}")
    return "; ".join(values)


def percentile(values: list[float], fraction: float) -> float:
    ordered = sorted(values)
    if not ordered:
        return 0.0
    position = (len(ordered) - 1) * fraction
    lower = int(position)
    upper = min(lower + 1, len(ordered) - 1)
    return ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)


class Adapter:
    def __init__(self, target: Target, insecure: bool) -> None:
        self.target = target
        self.context = ssl._create_unverified_context() if insecure else ssl.create_default_context()
        self.headers = self._auth_headers()
        self.item: dict[str, str] = {}

    def _auth_headers(self) -> dict[str, str]:
        kind = self.target.kind
        if kind == "kinosail":
            token = os.environ.get("KINOSAIL_BENCHMARK_TOKEN", "")
            if token:
                if len(token) > 4096:
                    raise ValueError("kinosail benchmark token is too large")
                return {"Authorization": f"Bearer {token}"}
            path = os.environ.get("KINOSAIL_BENCHMARK_COOKIE_FILE", "")
            return {"Cookie": cookie_header(path)} if path else {}
        token = os.environ.get(f"{kind.upper()}_BENCHMARK_TOKEN", "")
        if not token:
            return {}
        if len(token) > 4096:
            raise ValueError(f"{kind} benchmark token is too large")
        if kind == "jellyfin":
            return {"X-Emby-Token": token}
        return {"X-Plex-Token": token}

    def request(self, spec: RequestSpec) -> tuple[RequestResult, bytes]:
        headers = {"Accept": "application/json, application/xml, */*", **self.headers, **dict(spec.headers)}
        if self.target.kind == "plex" and "Accept" not in dict(spec.headers):
            headers["Accept"] = "application/xml"
        request = urllib.request.Request(self.target.base_url + spec.path, headers=headers)
        started = time.perf_counter()
        try:
            with urllib.request.urlopen(request, context=self.context, timeout=30) as response:
                ttfb_ms = (time.perf_counter() - started) * 1000
                body = response.read(MAX_BODY_BYTES + 1)
                result = RequestResult((time.perf_counter() - started) * 1000, response.status, len(body), ttfb_ms=ttfb_ms)
                return result, body[:MAX_BODY_BYTES]
        except urllib.error.HTTPError as error:
            ttfb_ms = (time.perf_counter() - started) * 1000
            body = error.read(MAX_BODY_BYTES + 1)
            return RequestResult((time.perf_counter() - started) * 1000, error.code, len(body), f"HTTP {error.code}", ttfb_ms), body[:MAX_BODY_BYTES]
        except (OSError, ValueError) as error:
            return RequestResult((time.perf_counter() - started) * 1000, 0, 0, type(error).__name__), b""

    def discover(self, range_sizes: tuple[int, ...]) -> dict[str, RequestSpec]:
        if not self.headers:
            raise ValueError(f"missing {self.target.kind} benchmark authentication")
        if self.target.kind == "kinosail":
            return self._discover_kinosail(range_sizes)
        if self.target.kind == "jellyfin":
            return self._discover_jellyfin(range_sizes)
        return self._discover_plex(range_sizes)

    def server_version(self) -> str | None:
        if self.target.kind == "kinosail":
            payload = self._decode_json(RequestSpec("/api/v1/openapi.json"))
            info = payload.get("info", {})
            return str(info.get("version")) if info.get("version") else None
        if self.target.kind == "jellyfin":
            payload = self._decode_json(RequestSpec("/System/Info/Public"))
            return str(payload.get("Version")) if payload.get("Version") else None
        result, body = self.request(RequestSpec("/identity"))
        if result.status < 200 or result.status >= 300:
            return None
        try:
            return ET.fromstring(body).attrib.get("version")
        except ET.ParseError:
            return None

    def _decode_json(self, spec: RequestSpec) -> dict[str, Any]:
        result, body = self.request(spec)
        if result.status < 200 or result.status >= 300:
            raise ValueError(f"discovery request failed: HTTP {result.status}")
        try:
            return json.loads(body)
        except json.JSONDecodeError as error:
            raise ValueError("discovery returned invalid JSON") from error

    def _discover_kinosail(self, range_sizes: tuple[int, ...]) -> dict[str, RequestSpec]:
        path = "/api/v1/library?view=movies&limit=100"
        payload = self._decode_json(RequestSpec(path))
        items = payload.get("items")
        if not isinstance(items, list) or not items:
            raise ValueError("Kinosail returned no movie items")
        item = next((value for value in items if value.get("title") == "Example Movie"), items[0])
        item_id = str(item.get("id", ""))
        if not item_id:
            raise ValueError("Kinosail movie has no ID")
        self.item = {"id": item_id}
        specs = {
            "health": RequestSpec("/healthz"),
            "library": RequestSpec(path),
            "search": RequestSpec("/api/v1/library?view=movies&limit=100&q=Arrival"),
            "item": RequestSpec(f"/api/v1/items/{urllib.parse.quote(item_id)}"),
            "artwork": RequestSpec(f"/art/{urllib.parse.quote(item_id)}"),
            "playback_plan": RequestSpec(f"/api/v1/items/{urllib.parse.quote(item_id)}/playback"),
        }
        for size in range_sizes:
            specs[f"media_range_{size // 1024}k"] = RequestSpec(f"/media/{urllib.parse.quote(item_id)}", (("Range", f"bytes=0-{size - 1}"),))
        return specs

    def _discover_jellyfin(self, range_sizes: tuple[int, ...]) -> dict[str, RequestSpec]:
        user_id = os.environ.get("JELLYFIN_BENCHMARK_USER_ID", "")
        if not user_id or not re.fullmatch(r"[A-Za-z0-9-]{8,64}", user_id):
            raise ValueError("JELLYFIN_BENCHMARK_USER_ID must contain the authenticated user ID")
        query = urllib.parse.urlencode({"Recursive": "true", "IncludeItemTypes": "Movie", "Limit": "100", "Fields": "Path,MediaSources,ImageTags"})
        path = f"/Users/{urllib.parse.quote(user_id)}/Items?{query}"
        payload = self._decode_json(RequestSpec(path))
        items = payload.get("Items")
        if not isinstance(items, list) or not items:
            raise ValueError("Jellyfin returned no movie items")
        item = next((value for value in items if value.get("Name") == "Example Movie"), items[0])
        item_id = str(item.get("Id", ""))
        if not re.fullmatch(r"[A-Fa-f0-9]{8,64}", item_id):
            raise ValueError("Jellyfin movie has no valid ID")
        self.item = {"id": item_id}
        specs = {
            "health": RequestSpec("/System/Info/Public"),
            "library": RequestSpec(path),
            "search": RequestSpec(f"/Users/{urllib.parse.quote(user_id)}/Items?{urllib.parse.urlencode({'Recursive': 'true', 'SearchTerm': 'Arrival', 'IncludeItemTypes': 'Movie', 'Limit': '100'})}"),
            "item": RequestSpec(f"/Items/{item_id}"),
            "artwork": RequestSpec(f"/Items/{item_id}/Images/Primary"),
            "playback_plan": RequestSpec(f"/Items/{item_id}/PlaybackInfo?UserId={urllib.parse.quote(user_id)}&MaxStreamingBitrate=20000000"),
        }
        for size in range_sizes:
            specs[f"media_range_{size // 1024}k"] = RequestSpec(f"/Videos/{item_id}/stream?Static=true", (("Range", f"bytes=0-{size - 1}"),))
        return specs

    def _discover_plex(self, range_sizes: tuple[int, ...]) -> dict[str, RequestSpec]:
        sections_result, sections_body = self.request(RequestSpec("/library/sections"))
        if sections_result.status < 200 or sections_result.status >= 300:
            raise ValueError(f"Plex sections request failed: HTTP {sections_result.status}")
        try:
            sections = ET.fromstring(sections_body)
        except ET.ParseError as error:
            raise ValueError("Plex returned invalid XML") from error
        section = next((node for node in sections.findall("Directory") if node.attrib.get("type") == "movie"), None)
        if section is None or not section.attrib.get("key"):
            raise ValueError("Plex returned no movie library")
        section_key = section.attrib["key"]
        path = f"/library/sections/{urllib.parse.quote(section_key, safe='')}/all?type=1&limit=100"
        items_result, items_body = self.request(RequestSpec(path))
        if items_result.status < 200 or items_result.status >= 300:
            raise ValueError(f"Plex movie request failed: HTTP {items_result.status}")
        try:
            movies = ET.fromstring(items_body)
        except ET.ParseError as error:
            raise ValueError("Plex returned invalid movie XML") from error
        item = next(iter(movies.findall("Video")), None)
        if item is None or not item.attrib.get("ratingKey"):
            raise ValueError("Plex returned no movie items")
        item_id = item.attrib["ratingKey"]
        part = next((part for media in item.findall("Media") for part in media.findall("Part") if part.attrib.get("key")), None)
        part_key = part.attrib.get("key", "") if part is not None else ""
        parsed_part = urllib.parse.urlsplit(part_key)
        if len(part_key) > 2048 or not parsed_part.path.startswith("/") or parsed_part.scheme or parsed_part.netloc or parsed_part.query or parsed_part.fragment:
            raise ValueError("Plex movie has no playable media part")
        self.item = {"id": item_id}
        metadata_path = f"/library/metadata/{urllib.parse.quote(item_id, safe='')}"
        return {
            "health": RequestSpec("/identity"),
            "library": RequestSpec(path),
            "search": RequestSpec("/hubs/search?query=Arrival&includeMetadata=1&limit=100"),
            "item": RequestSpec(metadata_path),
            "artwork": RequestSpec(f"{metadata_path}/thumb"),
            "playback_plan": RequestSpec(f"{metadata_path}?checkFiles=1"),
            **{
                f"media_range_{size // 1024}k": RequestSpec(parsed_part.path, (("Range", f"bytes=0-{size - 1}"),))
                for size in range_sizes
            },
        }
