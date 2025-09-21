#!/usr/bin/env python3
import json, os, argparse
from random import choice, randint

BASE_DOCS = [
    {
        "title": "Risk Register 2025",
        "url": "https://intranet.example.com/docs/risk-register-2025",
        "snippets": [
            "Key risks include stuck pipe events and mud losses at Well B-07. Mitigation involves additional casing strings and improved mud circulation procedures.",
            "Weather-related standby time at offshore rigs is expected to rise during the monsoon season. Contingency planning is recommended.",
            "Equipment failure, particularly BOP control systems, remains a high-cost contributor to NPT. Vendor audits and preventive maintenance are key measures."
        ]
    },
    {
        "title": "HSSE Annual Report 2024",
        "url": "https://intranet.example.com/docs/hsse-annual-report-2024",
        "snippets": [
            "Lost Time Injuries (LTI) decreased by 12% compared to 2023, reflecting better safety culture.",
            "Near misses related to dropped objects remain frequent and need stronger controls.",
            "Environmental spills were contained within 1 hour on average, improving response KPIs."
        ]
    }
]

def generate(n=100):
    rows = []
    for i in range(n):
        base = choice(BASE_DOCS)
        doc_id = f"doc-{1000+i}"
        title = f"{base['title']} (Rev {randint(1,5)})"
        url = base["url"].replace(".com/", f".com/v{randint(1,5)}/")
        page = 1
        k = randint(2, len(base["snippets"]))  # pilih 2–3 snippet
        for snip in base["snippets"][:k]:
            rows.append({
                "doc_id": doc_id,
                "title": title,
                "url": url,
                "snippet": snip,
                "page_no": page
            })
            page += 1
    return rows

def main():
    ap = argparse.ArgumentParser(description="Generate dummy doc-chunks JSON")
    ap.add_argument("--out", default="tools/gen_dummy/sample/doc_chunks.json",
                    help="Output JSON file")
    ap.add_argument("--overwrite", action="store_true")
    ap.add_argument("--docs", type=int, default=100,
                    help="How many docs (default=100)")
    ap.add_argument("--chunks", type=int, default=None,
                    help="Force total doc-chunks (override docs count)")
    args = ap.parse_args()

    if os.path.exists(args.out) and not args.overwrite:
        print(f"[skip] {args.out} exists. Use --overwrite to regenerate.")
        return

    count = args.chunks if args.chunks else args.docs
    rows = generate(count)

    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    with open(args.out, "w") as f:
        json.dump(rows, f, indent=2)
    print(f"[ok] wrote {args.out} with {len(rows)} rows")

if __name__ == "__main__":
    main()
