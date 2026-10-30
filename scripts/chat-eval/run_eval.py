"""Run the chat evaluation against a local API and score answers against ground truth.

usage: python3 -I run_eval.py <api_base_url> <label> [comma-separated ids]

Needs the local Postgres container (PG_CONTAINER, default boundary_bytes_pg)
loaded with every league, and the API started with LEAGUES_SHOW_HIDDEN=true.
Ground truth is computed from the database on first run (truth.json); each
question's chat answer passes when its first row has the right player or team
and, where given, a value within 2%. Writes results_<label>.json. Gemini
rate limits and 503s are retried. Each run costs one Jev and up to two Gemini
calls per question.
"""
import json
import os
import subprocess
import sys
import time
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from questions import QUESTIONS  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
TRUTH_FILE = os.path.join(HERE, "truth.json")


def psql(sql):
    out = subprocess.run(
        ["podman", "exec", os.environ.get("PG_CONTAINER", "boundary_bytes_pg"), "psql", "-U", "postgres", "-d", "boundary_bytes", "-tA", "-F", "\t", "-c", sql],
        capture_output=True, text=True, check=True,
    ).stdout.strip()
    return out.split("\t") if out else [None, None]


def truth():
    if os.path.exists(TRUTH_FILE):
        return json.load(open(TRUTH_FILE))
    t = {}
    for qid, _, _, sql in QUESTIONS:
        name, value = psql(" ".join(sql.split()))
        t[qid] = {"name": name, "value": float(value) if value not in (None, "") else None}
    json.dump(t, open(TRUTH_FILE, "w"), indent=1)
    return t


def ask(base, question):
    """Ask once, retrying up to 3 times when the AI provider rate-limits."""
    for attempt in range(4):
        req = urllib.request.Request(base + "/api/text-to-sql", data=json.dumps({"question": question}).encode(),
                                     headers={"Content-Type": "application/json"})
        start = time.time()
        try:
            with urllib.request.urlopen(req, timeout=120) as r:
                body = json.loads(r.read())
        except urllib.error.HTTPError as e:
            body = json.loads(e.read() or b"{}")
        transient = any(t in str(body.get("error", "")).lower() for t in ("rate limit", "503", "high demand"))
        if not transient or attempt == 3:
            return body, time.time() - start
        time.sleep(30 * (attempt + 1))


def name_key(name):
    """Match on the most distinctive token: surname for players, full name for teams."""
    parts = name.split()
    return name.lower() if len(parts) > 2 else parts[-1].lower()


def score(t, body):
    rows = body.get("data") or []
    if not rows:
        return False, "no rows"
    row = rows[0]
    texts = " ".join(str(v).lower() for v in row.values() if isinstance(v, str))
    if t["name"] and name_key(t["name"]) not in texts:
        return False, f"first row lacks {t['name']!r}: {json.dumps(row)[:160]}"
    if t["value"] is not None:
        nums = []
        for v in row.values():
            try:
                nums.append(float(v))
            except (TypeError, ValueError):
                pass
        tol = max(1.0, abs(t["value"]) * 0.02)
        if not any(abs(n - t["value"]) <= tol for n in nums):
            return False, f"value {t['value']} not in first row {json.dumps(row)[:160]}"
    return True, ""


def main():
    base, label = sys.argv[1], sys.argv[2]
    only = set(sys.argv[3].split(",")) if len(sys.argv) > 3 else None
    out_file = os.path.join(HERE, f"results_{label}.json")
    previous = {r["id"]: r for r in json.load(open(out_file))} if only and os.path.exists(out_file) else {}
    t = truth()
    results = []
    for qid, cat, question, _ in QUESTIONS:
        if only and qid not in only:
            if qid in previous:
                results.append(previous[qid])
            continue
        body, secs = ask(base, question)
        ok, why = score(t[qid], body)
        meta = body.get("metadata") or {}
        results.append({"id": qid, "category": cat, "question": question, "pass": ok, "why": why,
                        "truth": t[qid], "routedBy": meta.get("routedBy"), "sql": meta.get("generatedSql") or meta.get("generatedSQL"),
                        "error": body.get("error"), "seconds": round(secs, 1)})
        time.sleep(2)  # stay under the Gemini per-minute quota
        print(f"{'PASS' if ok else 'FAIL'} {qid:4s} {meta.get('routedBy') or '-':6s} {secs:5.1f}s {question[:70]}" + ("" if ok else f"\n      -> {why or body.get('error')}"))
    json.dump(results, open(out_file, "w"), indent=1)
    by = {}
    for r in results:
        by.setdefault(r["category"], [0, 0])
        by[r["category"]][0] += r["pass"]
        by[r["category"]][1] += 1
    print("\n" + label + ": " + ", ".join(f"{c} {p}/{n}" for c, (p, n) in by.items()) + f" | total {sum(r['pass'] for r in results)}/{len(results)}")


main()
