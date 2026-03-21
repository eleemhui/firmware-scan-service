#!/usr/bin/env python3
"""
Concurrent load tester for firmware-scan-service.

Usage:
    python load_test.py [--url URL] [--requests N] [--concurrency N] [--seed N]

Requirements:
    pip install aiohttp
"""

import argparse
import asyncio
import hashlib
import json
import os
import random
import string
import time
from collections import Counter
from dataclasses import dataclass, field

import aiohttp

# ---------------------------------------------------------------------------
# Data generation
# ---------------------------------------------------------------------------

HARDWARE_MODELS = ["X1000", "X2000", "AX-100", "AX-200", "EdgePro", "NanoEdge", "CoreV3"]
COMPONENT_TYPES = ["bootloader", "kernel", "network-driver", "crypto-module", "sensor-fw", "display"]
VENDORS = ["AcmeCorp", "TechBase", "FirmwareInc", "EmbeddedSys", "CoreTech"]


def random_version() -> str:
    major = random.randint(1, 5)
    minor = random.randint(0, 12)
    patch = random.randint(0, 99)
    return f"{major}.{minor}.{patch}"


def random_hash() -> str:
    return hashlib.sha256(os.urandom(32)).hexdigest()


def random_string(length: int) -> str:
    return "".join(random.choices(string.ascii_lowercase + string.digits, k=length))


def build_metadata(target_bytes: int) -> dict:
    """
    Build a metadata dict whose JSON representation is approximately target_bytes.
    Components are added until the size threshold is reached.
    """
    meta = {
        "hardware_model": random.choice(HARDWARE_MODELS),
        "board_revision": f"rev{random.randint(1, 9)}",
        "additional_info": {
            "mfg_date": f"202{random.randint(0,5)}-{random.randint(1,12):02d}-{random.randint(1,28):02d}",
            "region": random.choice(["EU", "US", "APAC", "LATAM"]),
            "notes": random_string(random.randint(10, 40)),
        },
        "components": [],
    }

    # Keep adding components until we hit the target size.
    while True:
        component = {
            "type": random.choice(COMPONENT_TYPES),
            "vendor": random.choice(VENDORS),
            "version": random_version(),
            "hash": random_hash(),
            "signed": random.choice([True, False]),
            "extra": random_string(random.randint(8, 32)),
        }
        meta["components"].append(component)
        if len(json.dumps(meta).encode()) >= target_bytes:
            break

    return meta


def random_payload() -> dict:
    target_bytes = random.randint(100, 10_240)  # 100 B – 10 KB
    return {
        "device_id": f"device-{random_string(8)}",
        "firmware_version": random_version(),
        "binary_hash": random_hash(),
        "metadata": build_metadata(target_bytes),
    }


# ---------------------------------------------------------------------------
# HTTP worker
# ---------------------------------------------------------------------------

@dataclass
class Result:
    status: int
    elapsed_ms: float
    is_new: bool = False  # True when server returns 201


async def send_request(session: aiohttp.ClientSession, url: str) -> Result:
    payload = random_payload()
    t0 = time.monotonic()
    async with session.post(url, json=payload) as resp:
        elapsed = (time.monotonic() - t0) * 1000
        is_new = resp.status == 201
        return Result(status=resp.status, elapsed_ms=elapsed, is_new=is_new)


async def worker(queue: asyncio.Queue, session: aiohttp.ClientSession,
                 url: str, results: list[Result]) -> None:
    while True:
        try:
            queue.get_nowait()
        except asyncio.QueueEmpty:
            break
        result = await send_request(session, url)
        results.append(result)
        queue.task_done()


# ---------------------------------------------------------------------------
# Runner + reporting
# ---------------------------------------------------------------------------

async def run(url: str, total: int, concurrency: int) -> None:
    queue: asyncio.Queue = asyncio.Queue()
    for _ in range(total):
        queue.put_nowait(None)

    results: list[Result] = []

    connector = aiohttp.TCPConnector(limit=concurrency)
    timeout = aiohttp.ClientTimeout(total=30)

    print(f"Sending {total} requests to {url} with concurrency={concurrency} …\n")
    t_start = time.monotonic()

    async with aiohttp.ClientSession(connector=connector, timeout=timeout) as session:
        workers = [
            asyncio.create_task(worker(queue, session, url, results))
            for _ in range(concurrency)
        ]
        await asyncio.gather(*workers)

    elapsed_total = time.monotonic() - t_start
    print_report(results, elapsed_total)


def print_report(results: list[Result], elapsed_total: float) -> None:
    if not results:
        print("No results.")
        return

    status_counts = Counter(r.status for r in results)
    new_scans = sum(1 for r in results if r.is_new)
    dupes = sum(1 for r in results if r.status == 200)
    errors = sum(1 for r in results if r.status not in (200, 201))

    times = sorted(r.elapsed_ms for r in results)
    n = len(times)
    p50 = times[int(n * 0.50)]
    p95 = times[int(n * 0.95)]
    p99 = times[int(n * 0.99)]
    avg = sum(times) / n

    print("=" * 50)
    print(f"  Total requests  : {n}")
    print(f"  Completed in    : {elapsed_total:.2f}s  ({n / elapsed_total:.1f} req/s)")
    print()
    print("  HTTP status breakdown:")
    for status, count in sorted(status_counts.items()):
        label = {201: "new scan", 200: "duplicate (idempotent)", }.get(status, "error")
        print(f"    {status}  {count:>5}   ({label})")
    print()
    print(f"  New scans (201) : {new_scans}")
    print(f"  Duplicates (200): {dupes}")
    print(f"  Errors          : {errors}")
    print()
    print("  Latency (ms):")
    print(f"    avg  {avg:7.1f}")
    print(f"    p50  {p50:7.1f}")
    print(f"    p95  {p95:7.1f}")
    print(f"    p99  {p99:7.1f}")
    print(f"    min  {times[0]:7.1f}")
    print(f"    max  {times[-1]:7.1f}")
    print("=" * 50)


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(description="Firmware scan service load tester")
    parser.add_argument("--url", default="http://localhost:8080/v1/firmware-scans",
                        help="Target endpoint (default: http://localhost:8080/v1/firmware-scans)")
    parser.add_argument("--requests", type=int, default=100,
                        help="Total number of requests to send (default: 100)")
    parser.add_argument("--concurrency", type=int, default=10,
                        help="Number of concurrent workers (default: 10)")
    parser.add_argument("--seed", type=int, default=None,
                        help="Random seed for reproducible payloads")
    args = parser.parse_args()

    if args.seed is not None:
        random.seed(args.seed)

    asyncio.run(run(args.url, args.requests, args.concurrency))


if __name__ == "__main__":
    main()
