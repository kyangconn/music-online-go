#!/usr/bin/env python3
"""Music Online refresh-token smoke test against a running local server.

Uses curl with a cookie jar (handles HttpOnly cookies like a browser would).
"""
import json
import os
import subprocess
import sys

BASE = os.environ.get("SMOKE_BASE", "http://localhost:8080/api/v1")


def curl(method, path, jar=None, body=None, token=None, extra=()):
    cmd = ["curl", "-s", "-w", "\n%{http_code}", "-X", method]
    if jar:
        cmd += ["-b", jar, "-c", jar]
    if body is not None:
        cmd += ["-H", "Content-Type: application/json", "-d", json.dumps(body)]
    if token:
        cmd += ["-H", f"Authorization: Bearer {token}"]
    cmd += list(extra) + [BASE + path]
    out = subprocess.run(cmd, capture_output=True, text=True, check=True).stdout
    raw, _, code = out.rpartition("\n")
    return int(code), raw


def check(name, got, want):
    status = "PASS" if got == want else "FAIL"
    print(f"[{status}] {name}: got {got}, want {want}")
    return got == want


def main():
    ok = True

    # 1. register + login
    code, _ = curl("POST", "/users/register", body={"username": "smoke", "email": "smoke@test.com", "password": "password123"})
    ok &= check("register", code in (201, 409), True)

    jar = "/tmp/mo-cookies.txt"
    code, raw = curl("POST", "/users/login", jar=jar, body={"username": "smoke", "password": "password123"})
    ok &= check("login", code, 200)
    login = json.loads(raw)
    token = login["data"]["access_token"]
    ok &= check("login body has no refresh_token", "refresh_token" in login["data"], False)
    ok &= check("access token ttl 900s", login["data"]["expires_in"], 900)

    # 2. profile with access token
    code, _ = curl("GET", "/users/profile", token=token)
    ok &= check("profile with token", code, 200)

    # 3. refresh rotates cookie
    code, raw = curl("POST", "/users/refresh", jar=jar)
    ok &= check("refresh", code, 200)
    refreshed = json.loads(raw)
    ok &= check("refresh body has no refresh_token", "refresh_token" in refreshed["data"], False)

    # 4. old cookie replay: must NOT grant a new token. Keep a copy of the
    # pre-rotation cookie, rotate the live jar, then replay the stale copy.
    jar2 = "/tmp/mo-cookies2.txt"
    subprocess.run(["cp", jar, jar2], check=True)
    code, _ = curl("POST", "/users/refresh", jar=jar)  # rotate live cookie
    ok &= check("rotate", code, 200)
    code, _ = curl("POST", "/users/refresh", jar=jar2)  # stale cookie replay
    ok &= check("stale cookie replay rejected", code, 401)

    # 5. logout revokes session immediately
    code, raw = curl("POST", "/users/refresh", jar=jar)
    ok &= check("refresh before logout", code, 200)
    token2 = json.loads(raw)["data"]["access_token"]
    code, _ = curl("POST", "/users/logout", jar=jar, token=token2)
    ok &= check("logout", code, 200)
    code, _ = curl("GET", "/users/profile", token=token2)
    ok &= check("access token dead after logout", code, 401)
    code, _ = curl("POST", "/users/refresh", jar=jar)
    ok &= check("refresh dead after logout", code, 401)

    # 6. logout-all revokes every device
    code, raw = curl("POST", "/users/login", jar=jar, body={"username": "smoke", "password": "password123"})
    ok &= check("relogin", code, 200)
    tokenA = json.loads(raw)["data"]["access_token"]
    code, raw = curl("POST", "/users/login", jar=jar2, body={"username": "smoke", "password": "password123"})
    ok &= check("relogin second device", code, 200)
    tokenB = json.loads(raw)["data"]["access_token"]
    code, _ = curl("POST", "/users/logout-all", jar=jar, token=tokenA)
    ok &= check("logout-all", code, 200)
    code, _ = curl("GET", "/users/profile", token=tokenB)
    ok &= check("second device dead after logout-all", code, 401)

    print("SMOKE", "OK" if ok else "FAILED")
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
