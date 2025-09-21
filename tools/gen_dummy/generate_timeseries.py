#!/usr/bin/env python3
"""
FILENAME: generate_timeseries.py 

Generate demo CSVs:
- sample_timeseries.csv      (tag_id, ts_utc, value, quality)
- sample_prod_daily.csv      (date, well_id, gas_mmscfd)
- sample_drilling_events.csv (well_id, event_type, sub_cause, start_time, end_time, cost_usd)
- sample_hsse_incidents.csv  (category, description, event_time, location)
- sample_work_orders.csv     (wo_id, asset_id, area, priority, status, due_date)
- sample_ts_signal.csv       (tag_id, asset_id, tag_name, unit, description)
- sample_purchase_orders.csv (po_number, vendor, status, eta_date)
- sample_wells.csv           (well_id, name, area)
- sample_doc_chunks.csv      (doc_id, title, url, snippet, page_no)

Usage:
  python tools/gen_dummy/generate_timeseries.py --days 14 --overwrite
  python tools/gen_dummy/generate_timeseries.py --start "2025-09-01 00:00:00" --days 7
"""
import csv, os, argparse
from datetime import datetime, timedelta
from random import gauss, random, choice, randint

WELLS = {
    "WELL_A12": {"FLOW_A12": ("MMSCFD", 12.0, 0.05)},
    "WELL_B07": {"OIL_B07":  ("BOPD",   240.0, 0.03)},
    "WELL_C03": {"FLOW_C03": ("MMSCFD", 0.3,   0.50)},
    "WELL_D01": {"OIL_D01":  ("BOPD",   250.0, 0.04)},
    "WELL_D02": {"OIL_D02":  ("BOPD",   200.0, 0.04)},
    "WELL_E05": {"FLOW_E05": ("MMSCFD", 8.0,   0.06)},
    "WELL_F10": {"FLOW_F10": ("MMSCFD", 0.0,   0.10)},
}

TS_FILENAME         = "sample_timeseries.csv"
DAILY_FILENAME      = "sample_prod_daily.csv"
EVENTS_FILENAME     = "sample_drilling_events.csv"
HSSE_FILENAME       = "sample_hsse_incidents.csv"
WO_FILENAME         = "sample_work_orders.csv"
TS_SIGNAL_FILENAME  = "sample_ts_signal.csv"
PO_FILENAME         = "sample_purchase_orders.csv"
WELLS_FILENAME      = "sample_wells.csv"
DOC_CHUNKS_FILENAME = "sample_doc_chunks.csv"

NPT_CAUSES = [
    ("NPT", "Stuck Pipe"), ("NPT", "Mud Losses"), ("NPT", "BOP Repair"),
    ("NPT", "Rig Power Failure"), ("NPT", "Hole Cleaning"),
    ("NPT", "Cementing Delay"), ("NPT", "Directional Tools Failure"),
    ("NPT", "Weather Standby"),
]

HSSE_CATEGORIES = [
    "Near Miss", "First Aid", "Medical Treatment",
    "LTI", "Spill", "Fire", "Property Damage", "Security"
]
HSSE_LOCATIONS = [
    "Rig B07", "Well A12", "Well D01", "Workshop D01",
    "Tank Farm E05", "Warehouse", "Offshore Platform", "Onshore Site"
]

def gen_work_orders(n=50):
    areas = ["Onshore North", "Offshore East", "Onshore South", "Offshore West"]
    statuses = ["open", "in-progress", "closed"]
    today = datetime.utcnow().date()
    for i in range(n):
        due = today + timedelta(days=int(1 + 10*random()))
        yield (f"WO-{1000+i}", f"COMP-{i%100}", choice(areas),
               int(1 + 3*random()), choice(statuses), due.isoformat())

def gen_timeseries(start: datetime, minutes: int, quality_drop_ratio: float = 0.02):
    t = start
    for _ in range(minutes):
        for tags in WELLS.values():
            for tag_id, (_, base, jitter) in tags.items():
                val = max(0.0, gauss(base, base * jitter * 0.5))
                if random() < 0.001:
                    val *= (0.7 if random() < 0.5 else 1.3)
                q = 0 if random() < quality_drop_ratio else 1
                yield (tag_id, t.strftime('%Y-%m-%d %H:%M:%S'), round(val, 4), q)
        t += timedelta(minutes=1)

def gen_daily(start_date: datetime, days: int):
    """Gas hanya untuk tag MMSCFD. Khusus WELL_B07, set row hari-pertama = 3.2 (meniru seed SQL)."""
    d = start_date.date()
    first_day = True
    for _ in range(days):
        for well_id, tags in WELLS.items():
            unit, base, *_ = next(iter(tags.values()))
            if unit == "MMSCFD":
                val = max(0.0, gauss(base, base * 0.08))
            else:
                # default 0 untuk non-gas; override seed untuk WELL_B07 hari pertama
                val = 3.2 if (first_day and well_id == "WELL_B07") else 0.0
            yield (d.isoformat(), well_id, round(val, 4))
        d += timedelta(days=1)
        first_day = False

def gen_drilling_events(start_dt: datetime, days: int, avg_events_per_well_per_week: float = 1.0):
    p_daily = min(0.9, max(0.0, avg_events_per_well_per_week / 7.0))
    d = start_dt
    for _ in range(days):
        for well_id in WELLS.keys():
            if random() < p_daily:
                event_type, sub_cause = choice(NPT_CAUSES)
                start_hour, start_min = int(20*random()), int(60*random())
                start_time = d.replace(hour=start_hour, minute=start_min, second=0, microsecond=0)
                duration_hours = max(1.0, min(12.0, gauss(4.0, 2.0)))
                end_time = start_time + timedelta(hours=duration_hours)
                rate = 25000 + (35000 * random())
                cost = max(1000.0, gauss(rate * duration_hours, rate * 0.3))
                yield (well_id, event_type, sub_cause,
                       start_time.strftime('%Y-%m-%d %H:%M:%S'),
                       end_time.strftime('%Y-%m-%d %H:%M:%S'),
                       round(cost, 0))
        d += timedelta(days=1)

def gen_hsse_incidents(start_dt, days, avg_incidents_per_day=1.0):
    d = start_dt
    for _ in range(days):
        n = 0; p = avg_incidents_per_day
        if random() < min(1.0, p): n += 1
        if random() < max(0.0, p - 1.0): n += 1
        if random() < max(0.0, p - 2.0): n += 1
        for _i in range(n):
            cat, loc = choice(HSSE_CATEGORIES), choice(HSSE_LOCATIONS)
            hour, minute = int(24*random()), int(60*random())
            event_time = d.replace(hour=hour, minute=minute, second=0, microsecond=0)
            desc_map = {
                "Near Miss":"Dropped object avoided",
                "First Aid":"Minor cut during handling tools",
                "Medical Treatment":"Eye irritation from chemical splash",
                "LTI":"Slip and fall with >1 day lost time",
                "Spill":"Small diesel spill during fueling",
                "Fire":"Small electrical fire contained",
                "Property Damage":"Forklift bumped storage rack",
                "Security":"Unauthorized access attempt",
            }
            desc = desc_map.get(cat,"General HSSE incident")
            yield (cat, desc, event_time.strftime('%Y-%m-%d %H:%M:%S'), loc)
        d += timedelta(days=1)

def gen_ts_signal():
    for well_id, tags in WELLS.items():
        asset = well_id
        for tag_id, (unit, _base, _jit) in tags.items():
            yield (tag_id, asset, tag_id, unit, f"Auto signal for {well_id}")

def gen_purchase_orders(n=40):
    vendors = ["Baker","Halliburton","SLB","Weatherford","NOV","LocalSupplierA","LocalSupplierB"]
    statuses = ["created","approved","in-transit","delivered","closed"]
    today = datetime.utcnow().date()
    for i in range(n):
        po_number = f"PO-{20250000+i}"
        vendor = choice(vendors)
        status = choice(statuses)
        eta = (today + timedelta(days=randint(1,20))).isoformat()
        amount = randint(10000, 700000)   # random nilai order
        yield (po_number, vendor, status, eta, amount)

def gen_wells():
    # Contoh simple: “oil” untuk tag OIL_*, lainnya “gas”; WELL_F10 kita buat inactive
    for well_id in WELLS.keys():
        typ = "oil" if well_id.startswith("WELL_D") or well_id.startswith("OIL_") else "gas"
        status = "inactive" if well_id == "WELL_F10" else "active"
        yield (well_id, f"{well_id} Name", typ, status)



def gen_doc_chunks():
    """Memindahkan seed doc_chunks dari 0003_sample_data.sql ke CSV (tanpa kolom embedding)."""
    rows = [
        ("DOC-1001","Risk Register 2025 Rev 1","https://intranet.local/docs/risk-register-2025",
         "Key risks include stuck pipe events and mud losses at Well B07.",1),
        ("DOC-1001","Risk Register 2025 Rev 1","https://intranet.local/docs/risk-register-2025",
         "Weather-related standby time at offshore rigs during monsoon season.",2),
        ("DOC-1001","Risk Register 2025 Rev 1","https://intranet.local/docs/risk-register-2025",
         "Equipment failure in BOP control systems remains costly.",3),

        ("DOC-1002","HSSE Annual Report 2024","https://intranet.local/docs/hsse-annual-report-2024",
         "Lost Time Injuries (LTI) decreased by 12% compared to 2023.",1),
        ("DOC-1002","HSSE Annual Report 2024","https://intranet.local/docs/hsse-annual-report-2024",
         "Near misses related to dropped objects remain frequent.",2),
        ("DOC-1002","HSSE Annual Report 2024","https://intranet.local/docs/hsse-annual-report-2024",
         "Environmental spills were contained within 1 hour on average.",3),

        ("DOC-1003","Maintenance Strategy 2025","https://intranet.local/docs/maintenance-strategy-2025",
         "Preventive maintenance schedules updated for all offshore rigs.",1),
        ("DOC-1003","Maintenance Strategy 2025","https://intranet.local/docs/maintenance-strategy-2025",
         "Vendor audits required for all critical equipment.",2),
        ("DOC-1003","Maintenance Strategy 2025","https://intranet.local/docs/maintenance-strategy-2025",
         "Budget allocation increased by 15% for safety-critical spares.",3),

        ("DOC-1004","Emergency Response Plan","https://intranet.local/docs/emergency-response-plan",
         "Updated contact list for emergency teams at Well A12.",1),
        ("DOC-1004","Emergency Response Plan","https://intranet.local/docs/emergency-response-plan",
         "New procedures for offshore evacuation drills.",2),
        ("DOC-1004","Emergency Response Plan","https://intranet.local/docs/emergency-response-plan",
         "Firefighting training sessions scheduled quarterly.",3),

        ("DOC-1005","Production Forecast Q4 2025","https://intranet.local/docs/production-forecast-q4-2025",
         "Expected gas production increase at Well E05.",1),
        ("DOC-1005","Production Forecast Q4 2025","https://intranet.local/docs/production-forecast-q4-2025",
         "Oil production stable at Wells D01 and D02.",2),
        ("DOC-1005","Production Forecast Q4 2025","https://intranet.local/docs/production-forecast-q4-2025",
         "Potential decline at Well C03 due to water breakthrough.",3),

        ("DOC-1006","Drilling Program 2026","https://intranet.local/docs/drilling-program-2026",
         "Plan to drill 5 new wells in Onshore South area.",1),
        ("DOC-1006","Drilling Program 2026","https://intranet.local/docs/drilling-program-2026",
         "Budget for directional drilling tools increased.",2),
        ("DOC-1006","Drilling Program 2026","https://intranet.local/docs/drilling-program-2026",
         "Contingency for weather-related delays included.",3),

        ("DOC-1007","Safety Training Manual","https://intranet.local/docs/safety-training-manual",
         "All employees must complete annual HSSE induction.",1),
        ("DOC-1007","Safety Training Manual","https://intranet.local/docs/safety-training-manual",
         "Dropped object prevention training modules updated.",2),
        ("DOC-1007","Safety Training Manual","https://intranet.local/docs/safety-training-manual",
         "New PPE requirements for offshore operations.",3),

        ("DOC-1008","Incident Investigation Guide","https://intranet.local/docs/incident-investigation-guide",
         "Root cause analysis method standardized company-wide.",1),
        ("DOC-1008","Incident Investigation Guide","https://intranet.local/docs/incident-investigation-guide",
         "Use of digital tools for evidence collection recommended.",2),
        ("DOC-1008","Incident Investigation Guide","https://intranet.local/docs/incident-investigation-guide",
         "Lessons learned database updated monthly.",3),

        ("DOC-1009","Sustainability Report 2024","https://intranet.local/docs/sustainability-report-2024",
         "Carbon footprint reduced by 8% compared to 2023.",1),
        ("DOC-1009","Sustainability Report 2024","https://intranet.local/docs/sustainability-report-2024",
         "Investment in renewable energy projects ongoing.",2),
        ("DOC-1009","Sustainability Report 2024","https://intranet.local/docs/sustainability-report-2024",
         "Community engagement programs expanded.",3),

        ("DOC-1010","Audit Findings 2025","https://intranet.local/docs/audit-findings-2025",
         "Non-conformance noted in rig power backup systems.",1),
        ("DOC-1010","Audit Findings 2025","https://intranet.local/docs/audit-findings-2025",
         "Corrective action plans due by Q1 2026.",2),
        ("DOC-1010","Audit Findings 2025","https://intranet.local/docs/audit-findings-2025",
         "Follow-up audit scheduled for mid-2026.",3),
    ]
    for r in rows:
        yield r  # (doc_id, title, url, snippet, page_no)

def write_csv(path, headers, rows, overwrite):
    if os.path.exists(path) and not overwrite:
        print(f"[skip] {path} exists. Use --overwrite to regenerate.")
        return
    with open(path, 'w', newline='') as f:
        w = csv.writer(f); w.writerow(headers)
        for r in rows: w.writerow(r)
    print(f"[ok] wrote {path}")

def main():
    ap = argparse.ArgumentParser(description="Generate demo CSV")
    ap.add_argument("--start", default=datetime.utcnow().strftime('%Y-%m-%d 00:00:00'),
                    help="UTC start datetime (YYYY-MM-DD HH:MM:SS)")
    ap.add_argument("--days", type=int, default=7)
    ap.add_argument("--overwrite", action="store_true")
    ap.add_argument("--ts_out", default=TS_FILENAME)
    ap.add_argument("--daily_out", default=DAILY_FILENAME)
    ap.add_argument("--events_out", default=EVENTS_FILENAME)
    ap.add_argument("--events_weekly_avg", type=float, default=1.0)
    ap.add_argument("--hsse_out", default=HSSE_FILENAME)
    ap.add_argument("--hsse_daily_avg", type=float, default=1.0)
    ap.add_argument("--wo_out", default=WO_FILENAME)
    ap.add_argument("--wo_count", type=int, default=50)
    ap.add_argument("--ts_signal_out", default=TS_SIGNAL_FILENAME)
    ap.add_argument("--po_out", default=PO_FILENAME)
    ap.add_argument("--po_count", type=int, default=40)
    ap.add_argument("--wells_out", default=WELLS_FILENAME)
    ap.add_argument("--doc_chunks_out", default=DOC_CHUNKS_FILENAME)
    args = ap.parse_args()

    start_dt = datetime.strptime(args.start, '%Y-%m-%d %H:%M:%S')
    minutes = args.days * 24 * 60

    write_csv(args.ts_out, ["tag_id","ts_utc","value","quality"],
              gen_timeseries(start_dt, minutes), args.overwrite)
    write_csv(args.daily_out, ["date","well_id","gas_mmscfd"],
              gen_daily(start_dt, args.days), args.overwrite)
    write_csv(args.events_out, ["well_id","event_type","sub_cause","start_time","end_time","cost_usd"],
              gen_drilling_events(start_dt, args.days, args.events_weekly_avg), args.overwrite)
    write_csv(args.hsse_out, ["category","description","event_time","location"],
              gen_hsse_incidents(start_dt, args.days, args.hsse_daily_avg), args.overwrite)
    write_csv(args.wo_out, ["wo_id","asset_id","area","priority","status","due_date"],
              gen_work_orders(args.wo_count), args.overwrite)
    write_csv(args.ts_signal_out, ["tag_id","asset_id","tag_name","unit","description"],
              gen_ts_signal(), args.overwrite)
    write_csv(args.po_out,
          ["po_number","vendor","status","eta","amount"],
          gen_purchase_orders(args.po_count),
          args.overwrite)

    
    write_csv(args.doc_chunks_out, ["doc_id","title","url","snippet","page_no"],
              gen_doc_chunks(), args.overwrite)

    write_csv(args.wells_out, ["well_id","name","type","status"], gen_wells(), args.overwrite)


if __name__ == "__main__":
    main()
