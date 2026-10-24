#!/usr/bin/env python3
import asyncio
import json
import ssl
import sys
import time

LISTEN, CONTROL = int(sys.argv[1]), int(sys.argv[2])
UPSTREAM_HOST, UPSTREAM_PORT = sys.argv[3], int(sys.argv[4])
state = {"mode": "pass", "rate": 0, "dropped": 0, "connections": 0, "bytes": 0, "since": time.time()}
live = set()
events = []
budget = {"next": time.monotonic()}


def record(event):
    events.append({"t": round(time.time() - state["since"], 3), **event})
    del events[:-500]


async def pipe(reader, writer, count):
    try:
        while True:
            data = await reader.read(65536)
            if not data or state["mode"] == "drop":
                break
            if count and state["rate"]:
                now = time.monotonic()
                budget["next"] = max(now, budget["next"]) + len(data) / state["rate"]
                await asyncio.sleep(budget["next"] - now)
            if state["mode"] == "drop":
                break
            writer.write(data)
            await writer.drain()
            if count:
                state["bytes"] += len(data)
    except (ConnectionError, asyncio.CancelledError, ssl.SSLError):
        pass
    finally:
        writer.close()


async def client(reader, writer):
    if state["mode"] == "drop":
        state["dropped"] += 1
        writer.transport.abort()
        return
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    try:
        upstream_reader, upstream_writer = await asyncio.open_connection(UPSTREAM_HOST, UPSTREAM_PORT, ssl=context)
    except OSError:
        writer.transport.abort()
        return
    state["connections"] += 1
    pair = (writer, upstream_writer)
    live.add(pair)
    try:
        await asyncio.gather(pipe(reader, upstream_writer, False), pipe(upstream_reader, writer, True))
    finally:
        live.discard(pair)


async def control(reader, writer):
    line = (await reader.readline()).decode("latin-1").split()
    while (await reader.readline()) not in (b"\r\n", b"\n", b""):
        pass
    path = line[1] if len(line) > 1 else "/"
    parts = [part for part in path.split("/") if part]
    if parts[:1] == ["mode"] and len(parts) == 2 and parts[1] in ("pass", "drop"):
        state["mode"] = parts[1]
        record({"mode": parts[1]})
        if parts[1] == "drop":
            for pair in list(live):
                for side in pair:
                    side.transport.abort()
    elif parts[:1] == ["rate"] and len(parts) == 2 and parts[1].isdigit():
        state["rate"] = int(parts[1])
        record({"rate": state["rate"]})
    body = json.dumps({**state, "live": len(live), "events": events}).encode()
    writer.write(b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: %d\r\n\r\n" % len(body) + body)
    await writer.drain()
    writer.close()


async def main():
    proxy = await asyncio.start_server(client, "127.0.0.1", LISTEN)
    admin = await asyncio.start_server(control, "127.0.0.1", CONTROL)
    async with proxy, admin:
        await asyncio.gather(proxy.serve_forever(), admin.serve_forever())


asyncio.run(main())
