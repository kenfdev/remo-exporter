#!/usr/bin/env python3
import argparse
import collections
import http.server
import json
import os
import pathlib
import socket
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("binary", nargs="?")
    parser.add_argument("--image")
    args = parser.parse_args()
    if bool(args.binary) == bool(args.image):
        parser.error("provide a binary path or --image")
    container_name = "remo-smoke-" + uuid.uuid4().hex
    requests = collections.Counter()
    devices = [
        {"id": "online", "name": "Living room", "online": True,
         "temperature_offset": 0.5,
         "newest_events": {
             "te": {"val": 22.5, "created_at": "2026-01-01T00:00:00Z"},
             "hu": {"val": 50, "created_at": "2026-01-01T00:00:00Z"},
             "il": {"val": 100, "created_at": "2026-01-01T00:00:00Z"},
             "mo": {"val": 1, "created_at": "2026-01-01T00:00:00Z"},
         }},
        {"id": "offline", "name": "Offline", "online": False},
        {"id": "null", "name": "Unknown", "online": None},
        {"id": "missing", "name": "Old firmware"},
    ]
    appliances = [{"type": "EL_SMART_METER", "device": {
        "id": "meter", "name": "Meter", "temperature_offset": 0.5,
    }, "smart_meter": {"echonetlite_properties": [
        {"epc": epc, "val": value}
        for epc, value in [(211, "1"), (215, "6"), (224, "50851"),
                           (225, "1"), (227, "11"), (231, "568")]
    ]}}]

    class API(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            if self.headers.get("Authorization") != "Bearer smoke-test-token":
                self.send_error(401)
                return
            payload = {"/1/devices": devices, "/1/appliances": appliances}.get(self.path)
            if payload is None:
                self.send_error(404)
                return
            requests[self.path] += 1
            body = json.dumps(payload).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("X-Rate-Limit-Limit", "30")
            self.send_header("X-Rate-Limit-Remaining", "28")
            self.send_header("X-Rate-Limit-Reset", "1767225900")
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_):
            pass

    api = http.server.HTTPServer(("127.0.0.1", 0), API)
    thread = threading.Thread(target=api.serve_forever, daemon=True)
    thread.start()
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    env = os.environ.copy()
    env.pop("OAUTH_TOKEN_FILE", None)
    env.update(OAUTH_TOKEN="smoke-test-token", API_BASE_URL=f"http://127.0.0.1:{api.server_port}",
               PORT=str(port), CACHE_INVALIDATION_SECONDS="60", METRICS_PATH="/metrics")
    with tempfile.TemporaryFile(mode="w+") as log:
        if args.image:
            command = ["docker", "run", "--rm", "--name", container_name, "--network", "host"]
            for key in ("OAUTH_TOKEN", "API_BASE_URL", "PORT", "CACHE_INVALIDATION_SECONDS", "METRICS_PATH"):
                command.extend(["--env", f"{key}={env[key]}"])
            command.append(args.image)
        else:
            command = [str(pathlib.Path(args.binary).resolve())]
        process = subprocess.Popen(command, env=env, stdout=log, stderr=log)
        try:
            url = f"http://127.0.0.1:{port}/metrics"
            deadline = time.monotonic() + 10
            while True:
                try:
                    response = urllib.request.urlopen(url, timeout=2)
                    break
                except (urllib.error.URLError, TimeoutError):
                    if process.poll() is not None or time.monotonic() >= deadline:
                        log.seek(0)
                        raise AssertionError("exporter did not become ready:\n" + log.read())
                    time.sleep(0.05)
            with response:
                body = response.read().decode()
                content_type = response.headers.get("Content-Type", "")
                assert "text/plain" in content_type and "version=0.0.4" in content_type, content_type
            expected = {
                'remo_temperature{id="online",name="Living room"}': 22.5,
                'remo_humidity{id="online",name="Living room"}': 50,
                'remo_illumination{id="online",name="Living room"}': 100,
                'remo_motion{id="online",name="Living room"}': 1767225600,
                'remo_device_online{id="online",name="Living room"}': 1,
                'remo_device_online{id="offline",name="Offline"}': 0,
                'remo_normal_direction_cumulative_electric_energy{id="meter",name="Meter"}': 50851,
                'remo_reverse_direction_cumulative_electric_energy{id="meter",name="Meter"}': 11,
                'remo_coefficient{id="meter",name="Meter"}': 1,
                'remo_cumulative_electric_energy_unit_kilowatt_hour{id="meter",name="Meter"}': 0.1,
                'remo_cumulative_electric_energy_effective_digits{id="meter",name="Meter"}': 6,
                'remo_measured_instantaneous_energy_watt{id="meter",name="Meter"}': 568,
                'remo_x_rate_limit_limit': 30,
                'remo_x_rate_limit_remaining': 28,
                'remo_x_rate_limit_reset': 1767225900,
                'remo_http_requests_total{api="devices",code="200"}': 1,
                'remo_http_requests_total{api="appliances",code="200"}': 1,
            }
            samples = {line.rsplit(" ", 1)[0]: float(line.rsplit(" ", 1)[1])
                       for line in body.splitlines() if line and not line.startswith("#")}
            for metric, want in expected.items():
                assert samples.get(metric) == want, (metric, samples.get(metric), want)
            assert len([name for name in samples if name.startswith("remo_device_online{")]) == 2
            with urllib.request.urlopen(url, timeout=2) as second:
                assert second.status == 200
                cached = second.read().decode()
            assert requests == {"/1/devices": 1, "/1/appliances": 1}, requests
            for api_name in ("devices", "appliances"):
                assert f'remo_http_requests_total{{api="{api_name}",code="200"}} 1\n' in cached
            print("PASS exporter: legacy metrics, fractional offsets, online/null/absent, content type, cache")
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            if args.image:
                subprocess.run(["docker", "rm", "--force", container_name],
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
            api.shutdown()
            api.server_close()
            thread.join(timeout=5)


if __name__ == "__main__":
    main()
