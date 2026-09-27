"""laya_dev.py -- Laya System One decision model for day-to-day development.

Why this wrapper exists
-----------------------
`pip install laya` gives you a Python library, not a tool: no CLI, no server.
This adds the two shapes you actually need while coding:

  1. One-shot CLI, JSON in / JSON out  -> pipeable, scriptable, agent-friendly
  2. Local HTTP server speaking the TypeSafe System One shape
     (POST /v1/systemone) -> use it from Go, Node, or any HTTP client

Two traps this handles for you
------------------------------
* `Router()` defaults to CPU. On CPU a 4-question Chinese call takes ~3500 ms;
  on GPU it takes ~76 ms. It does NOT warn you. Default here is cuda, and the
  resolved device is always reported.
* The DSH launcher injects NO_PROXY containing "[::1]", which makes httpx raise
  `InvalidURL: Invalid port: ':1]'` before any request is made. Normalised below
  before torch/huggingface_hub import, so this works inside DSH sessions.

Usage
-----
  # one-shot, state + questions as JSON
  python laya_dev.py --state state.json --questions questions.json
  python laya_dev.py --state state.json --questions questions.json --device cpu
  echo '{"body":"退款"}' | python laya_dev.py --questions q.json --state -

  # local server
  python laya_dev.py --serve --port 8077
  curl -s localhost:8077/v1/systemone -H 'Content-Type: application/json' -d @req.json

Notes
-----
* `preload=True` keeps both checkpoints resident. Without it, every language
  switch rebuilds a model (7-10 s median reload). Cost: ~163 s startup.
* First run downloads ~2.3 GB of weights into ~/.cache/huggingface/hub.
* `laya` is Python-only. The Node package (@receptron/laya) is a separate
  ONNX runtime if you need it from TypeScript.
"""

from __future__ import annotations

import argparse
import json
import os
import sys

# Must happen before httpx/huggingface_hub import. See module docstring.
_FIXED_NO_PROXY = "localhost,127.0.0.1,::1"
os.environ["NO_PROXY"] = _FIXED_NO_PROXY
os.environ["no_proxy"] = _FIXED_NO_PROXY
os.environ.setdefault("HF_HUB_DISABLE_SYMLINKS_WARNING", "1")


def load_router(device: str, preload: bool):
    import laya
    import torch

    if device == "auto":
        device = "cuda" if torch.cuda.is_available() else "cpu"
    if device == "cuda" and not torch.cuda.is_available():
        print(
            "[laya_dev] WARNING: cuda requested but unavailable; falling back to CPU "
            "(expect ~45x slower).",
            file=sys.stderr,
        )
        device = "cpu"

    where = torch.cuda.get_device_name(0) if device == "cuda" else "host CPU"
    print(f"[laya_dev] device={device} ({where}) preload={preload}", file=sys.stderr)
    return laya.Router(preload=preload, device=device), device


def read_json_arg(value: str):
    """'-' means stdin, a path means that file, anything else is inline JSON."""
    if value == "-":
        return json.load(sys.stdin)
    if os.path.exists(value):
        with open(value, "r", encoding="utf-8") as fh:
            return json.load(fh)
    return json.loads(value)


def run_oneshot(args) -> int:
    state = read_json_arg(args.state)
    questions = read_json_arg(args.questions)
    router, device = load_router(args.device, preload=args.preload)
    try:
        result = router.predict(state, questions, model=args.model)
    finally:
        if not args.serve:
            try:
                router.unload()
            except Exception:
                pass
    result.setdefault("_dev", {})["device"] = device
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    return 0


def run_server(args) -> int:
    from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

    router, device = load_router(args.device, preload=True)

    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def _send(self, code: int, payload: dict):
            body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            self.send_response(code)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            if self.path.rstrip("/") in ("/health", "/healthz"):
                self._send(200, {"status": "ok", "device": device})
            else:
                self._send(404, {"error": "not found"})

        def do_POST(self):
            if self.path.rstrip("/") not in ("/v1/systemone", "/systemone"):
                self._send(404, {"error": "not found; POST /v1/systemone"})
                return
            try:
                length = int(self.headers.get("Content-Length") or 0)
                req = json.loads(self.rfile.read(length) or b"{}")
            except Exception as exc:
                self._send(400, {"error": f"bad JSON: {exc}"})
                return

            state = req.get("state")
            questions = req.get("questions")
            if state is None or not questions:
                self._send(400, {"error": "body must contain 'state' and 'questions'"})
                return
            try:
                # `model` here is a laya checkpoint name (english/multilingual/
                # typed-decisions), NOT a TypeSafe model id.
                result = router.predict(state, questions, model=req.get("model"))
            except Exception as exc:
                self._send(500, {"error": f"{type(exc).__name__}: {exc}"})
                return
            result.setdefault("_dev", {})["device"] = device
            self._send(200, result)

        def log_message(self, fmt, *a):
            sys.stderr.write("[laya_dev] " + (fmt % a) + "\n")

    srv = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"[laya_dev] serving on http://{args.host}:{args.port}/v1/systemone", file=sys.stderr)
    try:
        srv.serve_forever()
    except KeyboardInterrupt:
        print("\n[laya_dev] stopped", file=sys.stderr)
    finally:
        srv.server_close()
        try:
            router.unload()
        except Exception:
            pass
    return 0


def main(argv=None) -> int:
    p = argparse.ArgumentParser(
        prog="laya_dev",
        description="Laya System One decisions for local development.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument("--state", help="JSON file, inline JSON, or '-' for stdin")
    p.add_argument("--questions", help="JSON file, inline JSON, or '-' for stdin")
    p.add_argument("--device", default="cuda", choices=["cuda", "cpu", "auto"],
                   help="default cuda; 'auto' picks cuda when available (default: cuda)")
    p.add_argument("--model", default=None,
                   help="force a checkpoint: english | multilingual | typed-decisions")
    p.add_argument("--preload", action="store_true",
                   help="keep all checkpoints resident (recommended when serving)")
    p.add_argument("--serve", action="store_true", help="run as a local HTTP server")
    p.add_argument("--host", default="127.0.0.1")
    p.add_argument("--port", type=int, default=8077)
    args = p.parse_args(argv)

    if args.serve:
        return run_server(args)
    if not args.state or not args.questions:
        p.error("--state and --questions are required unless --serve is used")
    return run_oneshot(args)


if __name__ == "__main__":
    raise SystemExit(main())
