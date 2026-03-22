#!/usr/bin/env python3
"""
Concurrent load tester for firmware-scan-service.

Usage:
    python load_test.py [--url URL] [--requests N] [--concurrency N] [--seed N]
                        [--pool-size N] [--duplicate-rate F] [--burst-rate F]

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
from dataclasses import dataclass

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


def version_hash(version: str) -> str:
    """Deterministic SHA256 for a firmware version — same version always yields the same hash."""
    return hashlib.sha256(version.encode()).hexdigest()


def random_payload() -> dict:
    version = random_version()
    return {
        "device_id": f"device-{random_string(8)}",
        "firmware_version": version,
        "binary_hash": version_hash(version),
        "metadata": build_metadata(random.randint(100, 10_240)),
    }


def build_pool(size: int) -> list[dict]:
    """Pre-generate a pool of unique payloads to reuse as duplicates."""
    return [random_payload() for _ in range(size)]


def build_queue_payloads(
    total: int,
    pool: list[dict],
    duplicate_rate: float,
    burst_rate: float,
) -> list[dict]:
    """
    Build the ordered list of payloads to send.

    - duplicate_rate  : probability any single slot picks from the pool (sequential dupe)
    - burst_rate      : probability of injecting a concurrent burst (2–4 copies of the
                        same pool payload added consecutively so workers race on them)
    """
    payloads: list[dict] = []
    while len(payloads) < total:
        remaining = total - len(payloads)

        if random.random() < burst_rate:
            # Concurrent burst: same payload queued multiple times back-to-back.
            burst_size = min(random.randint(2, 4), remaining)
            payload = random.choice(pool)
            payloads.extend([payload] * burst_size)

        elif random.random() < duplicate_rate:
            # Sequential duplicate: reuse a payload from the pool.
            payloads.append(random.choice(pool))

        else:
            # Fresh unique payload.
            payloads.append(random_payload())

    return payloads[:total]


# ---------------------------------------------------------------------------
# HTTP worker
# ---------------------------------------------------------------------------

@dataclass
class Result:
    status: int
    elapsed_ms: float
    is_new: bool = False


async def send_request(session: aiohttp.ClientSession, url: str, payload: dict) -> Result:
    t0 = time.monotonic()
    async with session.post(url, json=payload) as resp:
        elapsed = (time.monotonic() - t0) * 1000
        return Result(status=resp.status, elapsed_ms=elapsed, is_new=resp.status == 201)


async def worker(
    queue: asyncio.Queue,
    session: aiohttp.ClientSession,
    url: str,
    results: list[Result],
) -> None:
    while True:
        try:
            payload = queue.get_nowait()
        except asyncio.QueueEmpty:
            break
        result = await send_request(session, url, payload)
        results.append(result)
        queue.task_done()


# ---------------------------------------------------------------------------
# Runner + reporting
# ---------------------------------------------------------------------------

async def run(
    url: str,
    total: int,
    concurrency: int,
    pool_size: int,
    duplicate_rate: float,
    burst_rate: float,
) -> None:
    pool = build_pool(pool_size)
    payloads = build_queue_payloads(total, pool, duplicate_rate, burst_rate)

    queue: asyncio.Queue = asyncio.Queue()
    for p in payloads:
        queue.put_nowait(p)

    results: list[Result] = []
    connector = aiohttp.TCPConnector(limit=concurrency)
    timeout = aiohttp.ClientTimeout(total=30)

    dupe_count = total - len({id(p) for p in payloads})
    print(f"Sending {total} requests to {url}")
    print(f"  concurrency={concurrency}  pool={pool_size}  "
          f"duplicate_rate={duplicate_rate:.0%}  burst_rate={burst_rate:.0%}")
    print(f"  ~{dupe_count} duplicate payloads queued\n")

    t_start = time.monotonic()
    async with aiohttp.ClientSession(connector=connector, timeout=timeout) as session:
        workers = [
            asyncio.create_task(worker(queue, session, url, results))
            for _ in range(concurrency)
        ]
        await asyncio.gather(*workers)

    print_report(results, time.monotonic() - t_start)


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
    avg = sum(times) / n

    print("=" * 50)
    print(f"  Total requests  : {n}")
    print(f"  Completed in    : {elapsed_total:.2f}s  ({n / elapsed_total:.1f} req/s)")
    print()
    print("  HTTP status breakdown:")
    for status, count in sorted(status_counts.items()):
        label = {201: "new scan", 200: "duplicate (idempotent)"}.get(status, "error")
        print(f"    {status}  {count:>5}   ({label})")
    print()
    print(f"  New scans (201) : {new_scans}")
    print(f"  Duplicates (200): {dupes}")
    print(f"  Errors          : {errors}")
    print()
    print("  Latency (ms):")
    print(f"    avg  {avg:7.1f}")
    print(f"    p50  {times[int(n * 0.50)]:7.1f}")
    print(f"    p95  {times[int(n * 0.95)]:7.1f}")
    print(f"    p99  {times[int(n * 0.99)]:7.1f}")
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
    parser.add_argument("--pool-size", type=int, default=20,
                        help="Number of unique payloads in the reuse pool (default: 20)")
    parser.add_argument("--duplicate-rate", type=float, default=0.3,
                        help="Probability of sending a sequential duplicate (default: 0.3)")
    parser.add_argument("--burst-rate", type=float, default=0.1,
                        help="Probability of injecting a concurrent duplicate burst (default: 0.1)")
    args = parser.parse_args()

    if args.seed is not None:
        random.seed(args.seed)

    asyncio.run(run(
        args.url,
        args.requests,
        args.concurrency,
        args.pool_size,
        args.duplicate_rate,
        args.burst_rate,
    ))


if __name__ == "__main__":
    main()