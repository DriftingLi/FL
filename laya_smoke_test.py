"""Laya smoke test: Chinese + English routing, all three question primitives.

Run:  D:\\Anaconda\\python.exe D:\\FL\\laya_smoke_test.py
"""
import json
import time
import sys
import os

# The DSH launcher injects NO_PROXY containing "[::1]", which httpx rejects with
# "Invalid port: ':1]'" (bracketed IPv6 is not valid no_proxy syntax). httpx builds
# a URLPattern from every no_proxy entry, so this crashes client construction before
# any request is made. Normalise it here, before httpx/huggingface_hub import.
_fixed = "localhost,127.0.0.1,::1"
os.environ["NO_PROXY"] = _fixed
os.environ["no_proxy"] = _fixed

def main():
    print("=" * 62)
    print("Laya smoke test")
    print("=" * 62)

    import laya
    print("laya module :", getattr(laya, "__file__", "?"))
    try:
        import importlib.metadata as md
        print("laya version:", md.version("laya"))
        print("torch       :", md.version("torch"))
        print("transformers:", md.version("transformers"))
    except Exception as e:
        print("version lookup failed:", e)

    import torch
    print("cuda available:", torch.cuda.is_available())
    device = "cuda" if torch.cuda.is_available() else "cpu"
    print("using device  :", device)
    if device == "cuda":
        print("gpu           :", torch.cuda.get_device_name(0))

    # Three primitives at once, exactly like the Jev shape.
    questions = {
        "department": {
            "type": "choice",
            "instructions": "Which department should handle this request?",
            "criteria": {
                "billing": "invoices, payments, refunds",
                "technical": "bugs, outages, system errors",
                "sales": "pricing, new contracts",
                "other": "everything else",
            },
        },
        "urgency": {
            "type": "score",
            "instructions": "How urgent is this request?",
            "criteria": ["not urgent", "soon", "critical deadline or blocking issue"],
        },
        "refund_requested": {
            "type": "noul",
            "instructions": "Does the user explicitly request a refund?",
        },
        "churn_risk": {
            "type": "noul",
            "instructions": "Does the user threaten to cancel or leave?",
        },
    }

    samples = {
        "zh-CN (Chinese)": {
            "subject": "发票重复扣款",
            "body": "你好，我们三月份被重复扣了两次费用。请今天把多收的钱退回来，否则我们就取消订阅。",
        },
        "en (English)": {
            "from": "user@acme.com",
            "subject": "Duplicate charge on invoice #4411",
            "body": "Hi, we were billed twice for March. Please refund the duplicate today or we will cancel our plan.",
        },
    }

    print("\n--- loading Router (preload=True, device=%s) ---" % device)
    t0 = time.time()
    try:
        router = laya.Router(preload=True, device=device)
    except TypeError:
        # older signature fallback
        router = laya.Router()
    print("load took %.1f s" % (time.time() - t0))

    for label, state in samples.items():
        print("\n" + "=" * 62)
        print("STATE:", label)
        print("=" * 62)
        # warm-up (first call may build/compile kernels)
        try:
            router.predict(state, questions)
        except Exception as e:
            print("PREDICT FAILED:", type(e).__name__, e)
            continue

        timings = []
        res = None
        for _ in range(5):
            t0 = time.time()
            res = router.predict(state, questions)
            timings.append((time.time() - t0) * 1000)
        timings.sort()
        print("latency: min %.0f / median %.0f / max %.0f ms  (5 runs, %d questions)"
              % (timings[0], timings[len(timings) // 2], timings[-1], len(questions)))
        routing = res.get("routing")
        if routing:
            print("routing:", json.dumps(routing, ensure_ascii=False))
        print("usage  :", res.get("usage"))

        answers = res.get("answers", {})
        for qid, ans in answers.items():
            t = ans.get("type")
            if t == "choice":
                probs = {k: round(v, 4) for k, v in (ans.get("probabilities") or {}).items()}
                print("  %-18s choice=%-10s conf=%-6s probs=%s"
                      % (qid, ans.get("choice"), round(ans.get("confidence", -1), 3), probs))
            elif t == "score":
                print("  %-18s score=%-8s conf=%-6s legend=%s"
                      % (qid, round(ans.get("score", -1), 4), round(ans.get("confidence", -1), 3),
                         ans.get("legend")))
            elif t == "noul":
                print("  %-18s noul=%s" % (qid, round(ans.get("noul", -1), 4)))
            else:
                print("  %-18s %s" % (qid, ans))

    try:
        router.unload()
    except Exception:
        pass
    print("\nDONE")


if __name__ == "__main__":
    sys.exit(main())
