# tools/gen_dummy/generate_workorders.py
# Generator data dummy Work Orders JSON

# FILENAME: generate_workorders.py 

import json, random
from datetime import datetime, timedelta

def generate(n=5):
    rows = []
    for i in range(n):
        rows.append({
            "wo_id": f"WO-{1000+i}",
            "asset_id": f"COMP-{i}",
            "area": random.choice(["Onshore North","Offshore East"]),
            "priority": random.randint(1,3),
            "status": random.choice(["open","in-progress","closed"]),
            "due_date": (datetime.utcnow() + timedelta(days=random.randint(1,10))).strftime("%Y-%m-%d")
        })
    return rows

if __name__ == "__main__":
    fn = "tools/gen_dummy/sample/workorders.json"
    with open(fn,"w") as f:
        json.dump(generate(5), f, indent=2)
    print("wrote", fn)
