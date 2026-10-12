#!/usr/bin/env python3
"""harscan - stdlib-only HAR inspector (streaming, handles multi-hundred-MB files).

Commands
--------
  index   [--mime SUBSTR] [--status CODE] [--method M] [--grep PAT] [--json]
          Print one line per entry: idx method status mime size hasBody url.
  dump    --idx N [--out FILE] [--body request|response]
          Write a single entry's body to stdout (or FILE).
  dump-all --mime SUBSTR --out DIR [--pattern P] [--ext MAP]
          Write every matching response body to DIR/<idx>-<name>.
  assets  [--kind js|css|json|all]
          List bundle/asset URLs (deduped) with HTTP mime.
  grep    PATTERN [--in url|body|request|all] [--mime SUBSTR] [--context N] [--max N]
          Search entries (and bodies) for a regex.
  slim    --out FILE [--keep mime,..] [--max-body BYTES]
          Write a HAR copy with images/base64/oversize bodies stripped.

The file is parsed incrementally so a 200 MB+ HAR never has to be fully in RAM.
"""
import argparse
import base64
import json
import os
import re
import sys

ENTRY_MARKER = '"entries"'
CHUNK = 1 << 20
MAX_ENTRY_LOOKAHEAD = 1 << 31


def iter_entries(path):
    """Yield HAR log entries without loading the whole file."""
    dec = json.JSONDecoder()
    with open(path, encoding="utf-8") as f:
        buf = ""
        while ENTRY_MARKER not in buf:
            more = f.read(CHUNK)
            if not more:
                raise ValueError("no entries array found in %s" % path)
            buf += more
        i = buf.index(ENTRY_MARKER) + len(ENTRY_MARKER)
        j = buf.index("[", i)
        pos = j + 1
        while True:
            while True:
                while pos < len(buf) and buf[pos] in " \t\r\n,":
                    pos += 1
                if pos >= len(buf):
                    break
                if buf[pos] == "]":
                    return
                try:
                    obj, end = dec.raw_decode(buf, pos)
                except ValueError:
                    break
                yield obj
                pos = end
                if pos > (8 << 20):
                    buf = buf[pos:]
                    pos = 0
            more = f.read(CHUNK)
            if not more:
                return
            buf += more


def _mime(entry):
    return (entry.get("response", {}).get("content", {}) or {}).get("mimeType", "") or ""


def _body(entry, which="response"):
    part = entry.get(which, {})
    if which == "response":
        content = part.get("content", {}) or {}
        text = content.get("text", "") or ""
        if content.get("encoding") == "base64" and text:
            try:
                text = base64.b64decode(text).decode("utf-8", "replace")
            except Exception:
                pass
        return text
    pd = part.get("postData", {}) or {}
    return pd.get("text", "") or ""


def _size(entry, which="response"):
    part = entry.get(which, {})
    if which == "response":
        c = part.get("content", {}) or {}
        return c.get("size", len(_body(entry, which)))
    return part.get("bodySize", len(_body(entry, which)))


def _url(entry):
    return entry.get("request", {}).get("url", "")


def _status(entry):
    return entry.get("response", {}).get("status", 0)


def _method(entry):
    return entry.get("request", {}).get("method", "")


def cmd_index(a):
    rows = []
    for idx, e in enumerate(iter_entries(a.har)):
        mime = _mime(e)
        url = _url(e)
        if a.mime and a.mime.lower() not in mime.lower():
            continue
        if a.status and a.status != _status(e):
            continue
        if a.method and a.method.upper() != _method(e).upper():
            continue
        if a.grep and not re.search(a.grep, url) and not re.search(a.grep, _body(e)):
            continue
        rows.append((idx, _method(e), _status(e), mime, _size(e), bool(_body(e)), url))
    if a.json:
        print(json.dumps([
            {"idx": r[0], "method": r[1], "status": r[2], "mime": r[3],
             "size": r[4], "hasBody": r[5], "url": r[6]} for r in rows], indent=2))
        return
    for idx, method, status, mime, size, has, url in rows:
        print("%5d %-4s %3d %-28s %9d %s %s" % (
            idx, method, status, mime[:28], size, "B" if has else "-", safe(url)))


def safe(u, n=160):
    u = u.replace("\n", " ")
    return u if len(u) <= n else u[:n] + "..."


def cmd_dump(a):
    for idx, e in enumerate(iter_entries(a.har)):
        if idx != a.idx:
            continue
        text = _body(e, a.body)
        if a.out:
            with open(a.out, "w", encoding="utf-8", newline="") as f:
                f.write(text)
            sys.stderr.write("wrote %s (%d bytes)\n" % (a.out, len(text)))
        else:
            sys.stdout.write(text)
            if text and not text.endswith("\n"):
                sys.stdout.write("\n")
        return
    sys.exit("entry %d not found" % a.idx)


def _guess_name(url, idx, mime, ext_map):
    from urllib.parse import urlsplit
    p = urlsplit(url)
    base = os.path.basename(p.path) or ("index_%d" % idx)
    base = re.sub(r"[^A-Za-z0-9._-]", "_", base)[:80]
    ext = ext_map.get(mime.split(";")[0].strip())
    if ext and not base.endswith(ext):
        base += ext
    return "%05d-%s" % (idx, base)


def cmd_dump_all(a):
    ext_map = {"application/json": ".json", "application/javascript": ".js",
               "text/javascript": ".js", "text/css": ".css", "text/html": ".html"}
    os.makedirs(a.out, exist_ok=True)
    n = 0
    for idx, e in enumerate(iter_entries(a.har)):
        mime = _mime(e)
        if a.mime.lower() not in mime.lower():
            continue
        if a.pattern and not re.search(a.pattern, _url(e)):
            continue
        text = _body(e)
        if not text or len(text) > a.max:
            continue
        name = _guess_name(_url(e), idx, mime, ext_map)
        with open(os.path.join(a.out, name), "w", encoding="utf-8", newline="") as f:
            f.write(text)
        n += 1
    sys.stderr.write("wrote %d files to %s\n" % (n, a.out))


def cmd_assets(a):
    seen = {}
    for idx, e in enumerate(iter_entries(a.har)):
        mime = _mime(e)
        url = _url(e)
        kind = "other"
        if "javascript" in mime:
            kind = "js"
        elif "css" in mime:
            kind = "css"
        elif "json" in mime:
            kind = "json"
        if a.kind != "all" and kind != a.kind:
            continue
        if url in seen:
            seen[url][2] += 1
            continue
        seen[url] = [kind, idx, 1, mime, _size(e)]
    for url, (kind, idx, cnt, mime, size) in seen.items():
        print("%-5s %5d x%-2d %9d %-26s %s" % (kind, idx, cnt, size, mime[:26], safe(url)))


def cmd_grep(a):
    rx = re.compile(a.pat)
    hits = 0
    for idx, e in enumerate(iter_entries(a.har)):
        if a.mime and a.mime.lower() not in _mime(e).lower():
            continue
        fields = []
        if a.in_ in ("all", "url"):
            fields.append(("url", _url(e)))
        if a.in_ in ("all", "body"):
            fields.append(("resp", _body(e)))
        if a.in_ in ("all", "request"):
            fields.append(("req", _body(e, "request")))
        for fname, text in fields:
            m = rx.search(text)
            if not m:
                continue
            s = max(0, m.start() - a.context)
            snippet = text[s:m.end() + a.context].replace("\n", " ")
            print("%5d %-4s %s [%s] ...%s..." % (
                idx, fname, safe(_url(e), 90), _mime(e)[:20], snippet[:240]))
            hits += 1
            if a.max and hits >= a.max:
                return


def cmd_slim(a):
    keep = set(x.strip() for x in a.keep.split(",") if x.strip()) if a.keep else set()
    entries = []
    for e in iter_entries(a.har):
        mime = _mime(e).split(";")[0].strip()
        c = e.get("response", {}).get("content", {})
        text = c.get("text", "") or ""
        if keep and mime not in keep:
            c["text"] = ""
        elif text.startswith("data:") or (a.max_body and len(text) > a.max_body):
            c["text"] = ""
        if "text" in c and not c["text"]:
            c["comment"] = (c.get("comment", "") + " [harscan slimmed]").strip()
        entries.append(e)
    out = {"log": {"version": "1.2", "creator": {"name": "harscan slim"}}}
    out["log"]["entries"] = entries
    with open(a.out, "w", encoding="utf-8", newline="") as f:
        json.dump(out, f)
    sys.stderr.write("wrote %s with %d entries\n" % (a.out, len(entries)))


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)

    def add_common(p):
        p.add_argument("har", help="path to .har")

    p = sub.add_parser("index")
    add_common(p)
    p.add_argument("--mime")
    p.add_argument("--status", type=int)
    p.add_argument("--method")
    p.add_argument("--grep")
    p.add_argument("--json", action="store_true")
    p.set_defaults(func=cmd_index)

    p = sub.add_parser("dump")
    add_common(p)
    p.add_argument("--idx", type=int, required=True)
    p.add_argument("--out")
    p.add_argument("--body", choices=["request", "response"], default="response")
    p.set_defaults(func=cmd_dump)

    p = sub.add_parser("dump-all")
    add_common(p)
    p.add_argument("--mime", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--pattern")
    p.add_argument("--max", type=int, default=32 << 20)
    p.set_defaults(func=cmd_dump_all)

    p = sub.add_parser("assets")
    add_common(p)
    p.add_argument("--kind", choices=["js", "css", "json", "all"], default="all")
    p.set_defaults(func=cmd_assets)

    p = sub.add_parser("grep")
    add_common(p)
    p.add_argument("pat")
    p.add_argument("--in", dest="in_", choices=["url", "body", "request", "all"], default="all")
    p.add_argument("--mime")
    p.add_argument("--context", type=int, default=60)
    p.add_argument("--max", type=int, default=0)
    p.set_defaults(func=cmd_grep)

    p = sub.add_parser("slim")
    add_common(p)
    p.add_argument("--out", required=True)
    p.add_argument("--keep", help="comma list of mime types to keep bodies for")
    p.add_argument("--max-body", type=int, default=0)
    p.set_defaults(func=cmd_slim)

    a = ap.parse_args()
    a.func(a)


if __name__ == "__main__":
    main()
