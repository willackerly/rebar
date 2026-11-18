#!/usr/bin/env python3
"""Regression tests for the ASK servers' request hardening (v3.1.2).

Run by ask-security.test.sh; standalone: python3 scripts/tests/ask_security_test.py

Fixture (ASK_SEC_FIXTURE, built by the shell test):
  repos/proj/agents/architect/      the only served repo/agent
  hidden/agents/secret/AGENT.md     an agent outside every served repo
  evil/x.sh                         writes evil/pwned when run
  bin/claude                        fake claude: prints a canned JSON answer

Each test drives a real server process (bin/ask-server, bin/ask-mcp-server)
whose --ask-bin is the real bin/ask, and covers one attack that worked
against v3.1.1:
  - a question that names a script by relative path ran that script
  - a role with ../ reached agents outside the served repos
  - Content-Length was trusted for allocation (no size cap)
  - default bind 0.0.0.0, no key required
  - text/plain POSTs (cross-site) and any Host header (DNS rebinding) served
  - API key compared with ==
"""

import importlib.machinery
import importlib.util
import json
import os
import shutil
import socket
import subprocess
import sys
import time
import unittest
import http.client

BIN = os.environ.get("ASK_SEC_BIN") or os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..", "bin"))
FX = os.environ.get("ASK_SEC_FIXTURE", "")
REPOS = os.path.join(FX, "repos")
PWNED = os.path.join(FX, "evil", "pwned")
HIDDEN_SESSION = os.path.join(FX, "hidden", "agents", "secret", ".session-id")
# From repos/proj/agents/architect/commands/ up to the fixture root.
CMD_TRAVERSAL = "../../../../../evil/x"
# From repos/proj/agents/ up to the fixture root.
ROLE_TRAVERSAL = "../../../hidden/agents/secret"
# bin/ask answers only when its own dependencies are present.
CAN_ANSWER = bool(shutil.which("jq") and shutil.which("uuidgen"))


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def load(name):
    """Import a bin/ script (no .py suffix) as a module."""
    path = os.path.join(BIN, name)
    loader = importlib.machinery.SourceFileLoader(name.replace("-", "_"), path)
    spec = importlib.util.spec_from_loader(loader.name, loader)
    mod = importlib.util.module_from_spec(spec)
    loader.exec_module(mod)
    return mod


class Server:
    """A server process on a free port; stdout/stderr captured."""

    def __init__(self, prog, *extra, expect_start=True):
        self.port = free_port()
        self.argv = [sys.executable, os.path.join(BIN, prog), "--port", str(self.port),
                     "--repos-dir", REPOS, "--ask-bin", os.path.join(BIN, "ask")] + list(extra)
        env = dict(os.environ, PYTHONUNBUFFERED="1")
        env.pop("ASK_API_KEY", None)
        self.proc = subprocess.Popen(self.argv, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                     cwd=FX, env=env, text=True)
        self.output = ""
        if expect_start:
            self._wait_listening()

    def _wait_listening(self):
        deadline = time.time() + 15
        while time.time() < deadline:
            line = self.proc.stdout.readline()
            if not line:
                break
            self.output += line
            if "listening on" in line:
                return
        self.stop()
        raise AssertionError(f"server did not start: {self.argv}\n{self.output}")

    def wait_exit(self, timeout=15):
        try:
            out, _ = self.proc.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            self.stop()
            raise AssertionError(f"server kept running: {self.argv}\n{self.output}")
        self.output += out
        return self.proc.returncode

    def stop(self):
        if self.proc.poll() is None:
            self.proc.kill()
        try:
            self.proc.communicate(timeout=5)
        except Exception:
            pass

    def request(self, method, path, body=None, headers=None, host="127.0.0.1"):
        conn = http.client.HTTPConnection(host, self.port, timeout=30)
        hdrs = {"Host": f"localhost:{self.port}"}
        if body is not None:
            hdrs["Content-Type"] = "application/json"
            if not isinstance(body, (bytes, str)):
                body = json.dumps(body)
        hdrs.update(headers or {})
        conn.request(method, path, body=body, headers=hdrs)
        resp = conn.getresponse()
        data = resp.read().decode("utf-8", "replace")
        conn.close()
        return resp.status, data


class Base(unittest.TestCase):
    PROG = "ask-server"

    def setUp(self):
        for p in (PWNED, HIDDEN_SESSION):
            if os.path.exists(p):
                os.remove(p)
        self.servers = []

    def tearDown(self):
        for s in self.servers:
            s.stop()

    def start(self, *extra, prog=None, expect_start=True):
        s = Server(prog or self.PROG, *extra, expect_start=expect_start)
        self.servers.append(s)
        return s


class AskServerTests(Base):
    PROG = "ask-server"

    def test_question_is_never_a_command(self):
        s = self.start()
        status, body = s.request("POST", "/v1/ask", {"repo": "proj", "role": "architect",
                                                     "question": CMD_TRAVERSAL})
        self.assertFalse(os.path.exists(PWNED), "question ran a script outside commands/")
        if CAN_ANSWER:
            self.assertEqual(status, 200, body)
            self.assertIn("FAKE-ANSWER", body)

    def test_command_word_question_is_asked_not_run(self):
        s = self.start()
        status, body = s.request("POST", "/v1/ask", {"repo": "proj", "role": "architect",
                                                     "question": "hello"})
        self.assertNotIn("HELLO-CMD", body, "question ran an agent command script")

    def test_role_traversal_rejected(self):
        s = self.start()
        status, body = s.request("POST", "/v1/ask", {"repo": "proj", "role": ROLE_TRAVERSAL,
                                                     "question": "hi"})
        self.assertEqual(status, 404, body)
        self.assertFalse(os.path.exists(HIDDEN_SESSION), "unserved agent was queried")
        for path in (f"/v1/log/proj/{ROLE_TRAVERSAL}", "/v1/log/proj/..%2F..%2Fx"):
            status, body = s.request("GET", path)
            self.assertNotIn("Secret", body)

    def test_unknown_repo_rejected(self):
        s = self.start()
        status, body = s.request("POST", "/v1/ask", {"repo": "hidden", "role": "secret",
                                                     "question": "hi"})
        self.assertEqual(status, 404, body)

    def raw_post(self, s, content_length, body=b"{"):
        """Send headers and a little body; return the status line. The
        server must answer without waiting for (or allocating) the rest."""
        sock = socket.create_connection(("127.0.0.1", s.port), timeout=10)
        try:
            sock.sendall(("POST /v1/ask HTTP/1.1\r\nHost: localhost\r\n"
                          "Content-Type: application/json\r\n"
                          f"Content-Length: {content_length}\r\n\r\n").encode() + body)
            return sock.recv(4096).decode("latin-1").split("\r\n", 1)[0]
        finally:
            sock.close()

    def test_oversized_content_length_413(self):
        s = self.start()
        line = self.raw_post(s, 140000000000000)
        self.assertIn(" 413 ", line)
        status, _ = s.request("GET", "/v1/health")
        self.assertEqual(status, 200, "server died after an oversized Content-Length")

    def test_body_over_cap_413(self):
        s = self.start()
        self.assertIn(" 413 ", self.raw_post(s, (1 << 20) + 1, b"x" * 4096))
        self.assertIn(" 400 ", self.raw_post(s, -1))
        self.assertIn(" 411 ", self.raw_post(s, "abc"))

    def test_non_json_post_415(self):
        s = self.start()
        body = json.dumps({"repo": "proj", "role": "architect", "question": "hi"})
        for ctype in ("text/plain", "application/x-www-form-urlencoded", None):
            headers = {"Content-Type": ctype} if ctype else {}
            conn = http.client.HTTPConnection("127.0.0.1", s.port, timeout=30)
            conn.putrequest("POST", "/v1/ask", skip_host=True, skip_accept_encoding=True)
            conn.putheader("Host", "localhost")
            for k, v in headers.items():
                conn.putheader(k, v)
            conn.putheader("Content-Length", str(len(body)))
            conn.endheaders(body.encode())
            resp = conn.getresponse()
            resp.read()
            conn.close()
            self.assertEqual(resp.status, 415, f"Content-Type {ctype!r} accepted")

    def test_bad_host_403(self):
        s = self.start()
        status, _ = s.request("GET", "/v1/agents", headers={"Host": "evil.example"})
        self.assertEqual(status, 403)
        status, _ = s.request("GET", "/v1/agents", headers={"Host": f"evil.example:{s.port}"})
        self.assertEqual(status, 403)
        for host in (f"localhost:{s.port}", f"127.0.0.1:{s.port}", "[::1]", "localhost"):
            status, _ = s.request("GET", "/v1/agents", headers={"Host": host})
            self.assertEqual(status, 200, host)

    def test_default_bind_is_loopback(self):
        s = self.start()
        self.assertIn(f"listening on 127.0.0.1:{s.port}", s.output)

    def test_public_bind_without_key_refused(self):
        s = self.start("--host", "0.0.0.0", expect_start=False)
        self.assertEqual(s.wait_exit(), 2, s.output)
        self.assertIn("refusing", s.output)

    def test_public_bind_with_key_or_insecure_flag(self):
        s = self.start("--host", "0.0.0.0", "--api-key", "k")
        status, _ = s.request("GET", "/v1/agents", headers={"Host": "ask.example:7232",
                                                            "Authorization": "Bearer k"})
        self.assertEqual(status, 200, "public bind should accept any Host")
        s2 = self.start("--host", "0.0.0.0", "--insecure-no-auth")
        self.assertIn("WARNING", s2.output)

    def test_api_key(self):
        s = self.start("--api-key", "s3cret")
        cases = [({}, 401), ({"Authorization": "Bearer wrong"}, 401),
                 ({"Authorization": "Bearer s3cret"}, 200),
                 ({"Authorization": "Bearer s3cretX"}, 401),
                 ({"Authorization": "bearer s3cret"}, 401),
                 ({"Authorization": "Bearer s3crét"}, 401)]
        for headers, want in cases:
            status, _ = s.request("GET", "/v1/agents", headers=headers)
            self.assertIn(status, (want, 403) if want == 401 else (want,), headers)


class AskMCPServerHTTPTests(AskServerTests):
    PROG = "ask-mcp-server"

    # ask-mcp-server has no /v1/log; resources/read is covered over stdio.
    def test_role_traversal_rejected(self):
        s = self.start()
        status, body = s.request("POST", "/v1/ask", {"repo": "proj", "role": ROLE_TRAVERSAL,
                                                     "question": "hi"})
        self.assertNotEqual(status, 200, body)
        self.assertFalse(os.path.exists(HIDDEN_SESSION), "unserved agent was queried")

    def test_unknown_repo_rejected(self):
        s = self.start()
        status, body = s.request("POST", "/v1/ask", {"repo": "hidden", "role": "secret",
                                                     "question": "hi"})
        self.assertNotEqual(status, 200, body)

    def test_mcp_endpoint_checks(self):
        s = self.start()
        req = {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}}
        status, _ = s.request("POST", "/mcp", req)
        self.assertEqual(status, 200)
        status, _ = s.request("POST", "/mcp", json.dumps(req), headers={"Content-Type": "text/plain"})
        self.assertEqual(status, 415)
        status, _ = s.request("POST", "/mcp", req, headers={"Host": "evil.example"})
        self.assertEqual(status, 403)


class AskMCPServerStdioTests(Base):
    def rpc(self, *requests):
        lines = "".join(json.dumps(r) + "\n" for r in requests)
        proc = subprocess.run(
            [sys.executable, os.path.join(BIN, "ask-mcp-server"), "--stdio",
             "--repos-dir", REPOS, "--ask-bin", os.path.join(BIN, "ask")],
            input=lines, capture_output=True, text=True, cwd=FX, timeout=120)
        return [json.loads(l) for l in proc.stdout.splitlines() if l.strip()]

    INIT = {"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": {}}

    def test_tools_call_question_is_never_a_command(self):
        replies = self.rpc(self.INIT, {"jsonrpc": "2.0", "id": 1, "method": "tools/call",
                                       "params": {"name": "ask_proj_architect",
                                                  "arguments": {"question": CMD_TRAVERSAL}}})
        self.assertFalse(os.path.exists(PWNED), "tools/call ran a script outside commands/")
        if CAN_ANSWER:
            self.assertIn("FAKE-ANSWER", json.dumps(replies[-1]))

    def test_resources_read_role_traversal(self):
        replies = self.rpc(self.INIT, {"jsonrpc": "2.0", "id": 1, "method": "resources/read",
                                       "params": {"uri": f"ask://agent/proj:{ROLE_TRAVERSAL}"}})
        self.assertIn("error", replies[-1], replies[-1])
        self.assertNotIn("Secret", json.dumps(replies[-1]))


class HelperTests(unittest.TestCase):
    def test_helpers(self):
        for prog in ("ask-server", "ask-mcp-server"):
            m = load(prog)
            for bad in ("", ".", "..", "../x", "a/b", "a\\b", "-x", ".hidden", "a..b",
                        "p:a", "a\x00b", "a\nb", None, 3):
                self.assertFalse(m.valid_name(bad), (prog, bad))
            for good in ("architect", "Arch-1", "fr_v2", "a.b"):
                self.assertTrue(m.valid_name(good), (prog, good))
            for h in ("localhost", "localhost:7232", "127.0.0.1:1", "127.9.9.9", "[::1]:7232",
                      "[::1]", None):
                self.assertTrue(m.host_header_is_loopback(h), (prog, h))
            for h in ("evil.example", "evil.example:7232", "localhost.evil.example",
                      "10.0.0.1:7232", "[2001:db8::1]:80", ""):
                self.assertFalse(m.host_header_is_loopback(h), (prog, h))
            self.assertTrue(m.api_key_matches("Bearer k1", "k1"))
            self.assertFalse(m.api_key_matches("Bearer k2", "k1"))
            self.assertFalse(m.api_key_matches("Bearer k\udcff", "k1"))
            self.assertFalse(m.api_key_matches(None, "k1"))
            with open(os.path.join(BIN, prog)) as f:
                src = f.read()
            self.assertIn("hmac.compare_digest", src)
            self.assertEqual(m.ask_command("ask", "architect", "q", True),
                             ["ask", "-v", "--question", "--", "architect", "q"])


if __name__ == "__main__":
    if not FX or not os.path.isdir(REPOS):
        print("ASK_SEC_FIXTURE not set; run scripts/tests/ask-security.test.sh", file=sys.stderr)
        sys.exit(2)
    unittest.main(verbosity=2)
