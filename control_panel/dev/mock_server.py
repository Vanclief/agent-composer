#!/usr/bin/env python3
"""Mock agc backend for developing the control panel without agc.

Serves the panel at /panel/ and fakes the /api/tasks, /api/specs and
/api/msg endpoints with in-memory data. No WebSocket: terminal sockets get a plain
404, so the panel shows its "closed" path.

    python3 control_panel/dev/mock_server.py [--port 8765]
    open http://127.0.0.1:8765/panel/
"""

import argparse
import json
import os
import re
import secrets
import threading
import time
from datetime import datetime, timedelta, timezone
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, unquote, urlparse

PANEL_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
KNOWN_SECRETS = {"CLAUDE_CODE_OAUTH_TOKEN", "GITHUB_TOKEN"}
NAME_RE = re.compile(r"^[a-z0-9][a-z0-9_-]{0,63}$")
CHANNEL_RE = re.compile(r"^#[a-z0-9][a-z0-9_-]{0,63}$")

lock = threading.Lock()
posted = threading.Condition(lock)


def now():
    return datetime.now(timezone.utc)


def iso(t):
    return t.isoformat().replace("+00:00", "Z")


def slugify(title):
    # Same rule as the server (and slugify in app.js).
    s = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
    if s[:1].isdigit():
        s = "task-" + s
    return s[:63].rstrip("-")


def new_task(name, spec, status, age, status_error="", title=""):
    t = now() - age
    return {
        "id": "task_" + secrets.token_hex(6),
        "name": name,
        "title": title or name,
        "spec": spec,
        "status": status,
        "status_error": status_error,
        "created_at": iso(t),
        "updated_at": iso(t),
        "_ready_at": 0.0,
    }


TASKS = {}
TERMINALS = {}  # task name -> list of {id, command, running}
MESSAGES = []
PARTICIPANTS = {}  # id -> {id, name, task, human, active}
SPECS = {}  # name -> yaml
NEXT_ID = [1]


def seed():
    base = {
        "image": "agc-agent",
        "cpus": 2,
        "memory": 4096,
        "repo": "github.com/vanclief/agent-composer",
        "ref": "master",
        "setup": "go mod download\n",
        "secrets": ["CLAUDE_CODE_OAUTH_TOKEN", "GITHUB_TOKEN"],
        "env": {"GOFLAGS": "-mod=mod"},
        "volumes": ["go-mod-cache:/home/agent/go/pkg/mod:rw"],
        "channels": ["#releases", "#alerts:read"],
    }
    TASKS["review-42"] = new_task("review-42", base, "running", timedelta(minutes=40), title="Review 42")
    # A task from before titles existed: title equals name.
    TASKS["docs-pass"] = new_task("docs-pass", dict(base, cpus=1, memory=2048, ref="docs"), "stopped", timedelta(days=2))
    TASKS["flaky-ci"] = new_task(
        "flaky-ci",
        {"image": "agc-agent", "cpus": 2, "memory": 4096},
        "crashed",
        timedelta(hours=3),
        "vm exited with status 137 (out of memory)",
        title="Flaky CI",
    )
    SPECS["backend-review"] = "image: agc-agent\ncpus: 2\nmemory: 4096\nrepo: github.com/vanclief/agent-composer\nref: master\n"
    TERMINALS["review-42"] = [
        {"id": "job_" + secrets.token_hex(4), "command": ["claude"], "running": True},
        {"id": "job_" + secrets.token_hex(4), "command": ["bash"], "running": False},
    ]
    reviewer = join_participant("reviewer", "review-42", False)
    franco = join_participant("franco", "", True)
    add_message("#review-42", reviewer, "Cloned the repo and ran the tests. 3 failures in workflow/.")
    first = MESSAGES[-1]["id"]
    add_message("#review-42", franco, "Look at the loop node first.", first)
    add_message("#releases", reviewer, "v0.4.0 tagged.")
    # The seeded human has left, so the panel can join as @franco.
    franco["active"] = False


def join_participant(name, task, human):
    pid = "p_" + secrets.token_hex(8)
    PARTICIPANTS[pid] = {"id": pid, "name": name, "task": task, "human": human, "active": True, "joined_at": iso(now())}
    return PARTICIPANTS[pid]


def add_message(channel, p, body, reply_to=None):
    m = {
        "id": NEXT_ID[0],
        "channel": channel,
        "from": p["name"],
        "participant_id": p["id"],
        "task": p["task"],
        "body": body,
        "created_at": iso(now()),
    }
    if reply_to:
        m["reply_to"] = reply_to
    NEXT_ID[0] += 1
    MESSAGES.append(m)
    posted.notify_all()
    return m


def view(task):
    # A started task reports "starting" for two seconds, like a VM booting.
    if task["status"] == "starting" and time.time() >= task["_ready_at"]:
        task["status"] = "running"
        task["updated_at"] = iso(now())
    return {k: v for k, v in task.items() if not k.startswith("_")}


class ApiError(Exception):
    def __init__(self, status, body):
        super().__init__(str(body))
        self.status = status
        self.body = body


def task_error(status, message):
    # Tasks API errors shaped {"error":{"message":...}} (contract guess).
    return ApiError(status, {"error": {"message": message}})


def msg_error(status, message):
    # Messaging errors shaped like messaging/httpapi: {"error":"..."}.
    return ApiError(status, {"error": message})


def find_task(name):
    task = TASKS.get(name)
    if not task:
        raise task_error(404, 'task "%s" not found' % name)
    return task


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=PANEL_DIR, **kwargs)

    def log_message(self, fmt, *args):
        pass

    # ---------- plumbing ----------

    def send_json(self, status, body):
        data = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def read_json(self):
        n = int(self.headers.get("Content-Length") or 0)
        if n == 0:
            return {}
        try:
            body = json.loads(self.rfile.read(n))
        except ValueError:
            raise msg_error(400, "request body must be a JSON object")
        if not isinstance(body, dict):
            raise msg_error(400, "request body must be a JSON object")
        return body

    def dispatch(self, method):
        url = urlparse(self.path)
        if not url.path.startswith("/api/"):
            return self.static(method, url)
        parts = [unquote(p) for p in url.path.split("/")[2:] if p]
        query = {k: v[-1] for k, v in parse_qs(url.query).items()}
        try:
            status, body = self.api(method, parts, query)
        except ApiError as e:
            status, body = e.status, e.body
        self.send_json(status, body)

    def static(self, method, url):
        if method != "GET":
            return self.send_json(405, {"error": "method not allowed"})
        if url.path in ("/", "/panel"):
            self.send_response(302)
            self.send_header("Location", "/panel/")
            self.end_headers()
            return
        if not url.path.startswith("/panel/"):
            return self.send_json(404, {"error": "not found"})
        self.path = url.path[len("/panel"):] or "/"
        return super().do_GET()

    def do_GET(self):
        self.dispatch("GET")

    def do_POST(self):
        self.dispatch("POST")

    def do_PUT(self):
        self.dispatch("PUT")

    def do_DELETE(self):
        self.dispatch("DELETE")

    # ---------- API ----------

    def api(self, method, parts, query):
        if parts and parts[0] == "tasks":
            return self.tasks_api(method, parts[1:])
        if parts and parts[0] == "specs":
            return self.specs_api(method, parts[1:])
        if parts and parts[0] == "msg":
            return self.msg_api(method, parts[1:], query)
        raise msg_error(404, "no such endpoint")

    def tasks_api(self, method, rest):
        body = self.read_json() if method == "POST" else {}
        with lock:
            if not rest and method == "GET":
                return 200, {"tasks": [view(t) for t in TASKS.values()]}
            if not rest and method == "POST":
                return self.create_task(body)
            name = rest[0]
            task = find_task(name)
            if len(rest) == 1 and method == "GET":
                return 200, view(task)
            if len(rest) == 1 and method == "DELETE":
                if task["status"] == "running":
                    raise task_error(409, 'task "%s" is running; stop it first' % name)
                del TASKS[name]
                TERMINALS.pop(name, None)
                return 200, {"deleted": True, "name": name}
            if len(rest) == 2 and method == "POST" and rest[1] in ("start", "stop"):
                return self.lifecycle(task, rest[1])
            if len(rest) >= 2 and rest[1] == "terminals":
                return self.terminals_api(method, task, rest[2:], body)
        raise task_error(404, "no such endpoint")

    def specs_api(self, method, rest):
        body = self.read_json() if method == "PUT" else {}
        with lock:
            if not rest and method == "GET":
                return 200, {"specs": [{"name": n, "yaml": SPECS[n]} for n in sorted(SPECS)]}
            if len(rest) != 1:
                raise task_error(404, "no such endpoint")
            name = slugify(rest[0])
            if method == "PUT":
                text = str(body.get("yaml") or "")
                # Stand-in for the server's YAML validation.
                if not re.search(r"^image:\s*\S", text, re.M):
                    raise task_error(400, "spec is invalid: image is required")
                if not name:
                    raise task_error(400, "spec name %r needs at least one letter or digit" % rest[0])
                SPECS[name] = text
                return 200, {"name": name, "yaml": text}
            if name not in SPECS:
                raise task_error(404, 'spec "%s" not found' % rest[0])
            if method == "GET":
                return 200, {"name": name, "yaml": SPECS[name]}
            if method == "DELETE":
                return 200, {"name": name, "yaml": SPECS.pop(name)}
        raise task_error(404, "no such endpoint")

    def create_task(self, body):
        title = str(body.get("title") or "").strip()
        name = str(body.get("name") or "") or slugify(title)
        spec = body.get("spec") or {}
        if not name:
            raise task_error(400, "title %r needs at least one letter or digit" % title)
        if not NAME_RE.match(name):
            raise task_error(400, "name %r is invalid: use 1-64 lowercase letters, digits, _ or -" % name)
        if name in TASKS:
            raise task_error(409, 'task "%s" already exists' % name)
        if not isinstance(spec, dict):
            raise task_error(400, "spec must be an object")
        for entry in spec.get("secrets") or []:
            secret = str(entry).split("@")[0]
            if secret not in KNOWN_SECRETS:
                raise task_error(
                    400,
                    "secret %s is not defined; add it to ~/.config/agc/secrets.env and try again" % secret,
                )
        for ch in spec.get("channels") or []:
            if not CHANNEL_RE.match(str(ch).removesuffix(":read")):
                raise task_error(400, 'channel "%s" is invalid: start with # and use lowercase letters, digits, _ or -' % ch)
        task = new_task(name, spec, "starting", timedelta(0), title=title)
        task["_ready_at"] = time.time() + 2
        TASKS[name] = task
        TERMINALS[name] = []
        return 200, view(task)

    def lifecycle(self, task, action):
        if action == "stop":
            if task["status"] != "running":
                raise task_error(409, 'task "%s" is %s, not running' % (task["name"], task["status"]))
            task["status"] = "stopped"
            for job in TERMINALS.get(task["name"], []):
                job["running"] = False
        else:
            if task["status"] == "running":
                raise task_error(409, 'task "%s" is already running' % task["name"])
            task["status"] = "starting"
            task["status_error"] = ""
            task["_ready_at"] = time.time() + 2
        task["updated_at"] = iso(now())
        return 200, view(task)

    def terminals_api(self, method, task, rest, body):
        view(task)
        if task["status"] != "running":
            raise task_error(409, 'task "%s" is %s; start it to use terminals' % (task["name"], task["status"]))
        jobs = TERMINALS.setdefault(task["name"], [])
        if not rest and method == "GET":
            return 200, {"terminals": jobs}
        if not rest and method == "POST":
            command = str(body.get("command") or "").strip()
            if not command:
                raise task_error(400, "command is required")
            job = {"id": "job_" + secrets.token_hex(4), "command": command.split(), "running": True}
            jobs.append(job)
            return 200, {"id": job["id"]}
        if len(rest) == 2 and rest[1] == "ws":
            raise task_error(404, "the mock server has no WebSocket support")
        raise task_error(404, "no such endpoint")

    def msg_api(self, method, rest, query):
        verb = rest[0] if rest else ""
        if method == "GET" and verb == "channels":
            with lock:
                return 200, {"channels": self.channels()}
        if method == "GET" and verb in ("read", "wait"):
            return self.read(verb, query)
        if method != "POST":
            raise msg_error(404, "no such endpoint")
        body = self.read_json()
        with lock:
            if verb == "join":
                return self.join(body)
            if verb == "leave":
                p = PARTICIPANTS.get(str(body.get("participant_id") or ""))
                if not p:
                    raise msg_error(404, "participant %s not found" % body.get("participant_id"))
                p["active"] = False
                return 200, {"participant_id": p["id"], "left": True}
            if verb == "post":
                return self.post(body)
        raise msg_error(404, "no such endpoint")

    def channels(self):
        names = {"#" + n for n in TASKS} | {m["channel"] for m in MESSAGES}
        out = []
        for ch in sorted(names):
            msgs = [m for m in MESSAGES if m["channel"] == ch]
            out.append({"channel": ch, "read_only": False, "last_id": msgs[-1]["id"] if msgs else 0, "count": len(msgs)})
        return out

    def join(self, body):
        name = str(body.get("name") or "").lstrip("@")
        if not NAME_RE.match(name):
            raise msg_error(400, "name @%s is invalid" % name)
        for p in PARTICIPANTS.values():
            if p["active"] and p["name"] == name:
                raise msg_error(409, "name @%s is already in use" % name)
        return 200, join_participant(name, "", True)

    def post(self, body):
        pid = str(body.get("participant_id") or "")
        p = PARTICIPANTS.get(pid)
        if not p:
            raise msg_error(404, "participant %s not found" % pid)
        if not p["active"]:
            raise msg_error(400, "participant @%s has left, join again" % p["name"])
        channel = str(body.get("channel") or "")
        if not CHANNEL_RE.match(channel):
            raise msg_error(400, 'channel "%s" is invalid: use # followed by 1-64 lowercase letters, digits, _ or -' % channel)
        text = str(body.get("body") or "")
        if not text.strip():
            raise msg_error(400, "body is required")
        return 200, add_message(channel, p, text, body.get("reply_to"))

    def read(self, verb, query):
        channel = query.get("channel", "")
        if not CHANNEL_RE.match(channel):
            raise msg_error(400, 'channel "%s" is invalid: use # followed by 1-64 lowercase letters, digits, _ or -' % channel)
        try:
            after = int(query.get("after") or 0)
            limit = int(query.get("limit") or 50)
        except ValueError:
            raise msg_error(400, "after and limit must be integers")
        timeout = 30.0
        if query.get("timeout"):
            m = re.fullmatch(r"(\d+(?:\.\d+)?)(ms|s|m)", query["timeout"])
            if not m:
                raise msg_error(400, "timeout must be a duration such as 30s")
            timeout = float(m.group(1)) * {"ms": 0.001, "s": 1, "m": 60}[m.group(2)]
        deadline = time.time() + timeout
        with lock:
            while True:
                found = [m for m in MESSAGES if m["channel"] == channel and m["id"] > after][: min(limit, 500)]
                if found or verb == "read":
                    break
                left = deadline - time.time()
                if left <= 0:
                    break
                posted.wait(left)
        cursor = found[-1]["id"] if found else after
        return 200, {"messages": found, "cursor": cursor}


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8765)
    args = parser.parse_args()
    with lock:
        seed()
    server = ThreadingHTTPServer((args.host, args.port), Handler)
    server.daemon_threads = True
    print("control panel mock on http://%s:%d/panel/" % (args.host, args.port))
    server.serve_forever()


if __name__ == "__main__":
    main()
