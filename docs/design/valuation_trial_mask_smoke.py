# Run at the FL repository root: python docs/design/valuation_trial_mask_smoke.py
# Offline smoke check with catalog baseline and synthetic cases only.
from pathlib import Path
from collections import defaultdict
from itertools import product
from math import exp
import re
data = Path("backend/migrations/000074_baseline.up.sql").read_text(encoding="utf-8")
def inserts(table):
    blocks = re.findall(r"^INSERT INTO " + table + r" VALUES\n([\s\S]*?)\;\n", data, re.M)
    def tokens(row):
        return [x[1:-1].replace("''", "'") if x.startswith("'") else x for x in re.findall(r"'(?:''|[^'])*'|[^,\s]+", row)]
    return [tokens(line.strip().lstrip("(").rstrip(",").rstrip(")")) for block in blocks for line in block.splitlines()]
def read_row(r):
    _, brand, vehicle, series, tonnage, config, mast, height, price, year, *_ = r
    return brand, vehicle, float(tonnage), series, config, mast, int(height), float(price), int(year)
rows = [read_row(row) for row in inserts("original_prices")]
brands = {r[1]: float(r[2]) for r in inserts("brands")}
powers = {r[1]: r[2] for r in inserts("vehicle_types")}
coeff = {r[1]: float(r[2]) for r in inserts("coefficient_configs")}
ratings = {r[1]: float(r[3]) for r in inserts("condition_ratings")}
cohorts = defaultdict(list)
for row in rows:
    cohorts[row[:3]].append(row)
assert len(rows) == 264 and len(cohorts) == 114
masks = {
   "spec": ([.95], ["B"], [True]),
   "spec_and_hours": ([1.0, .95, .90], ["B"], [True]),
   "spec_hours_condition_registration": ([1.0, .95, .90], ["B", "C"], [True, False]),
}
limits = [.20, .30, .40]
stats = {name: [0,0,0] for name in masks}
for row in rows:
    brand, vehicle, *_ = row
    factory_year = max(2021, row[-1])
    age = 2026 - factory_year
    assert age >= 0
    power = powers.get(vehicle, "combustion" if "内燃" in vehicle else "electric")
    lambda_ = coeff["lambda_" + power]
    kb = brands[brand]
    def score(price, kh, rating, registered):
        kc = ratings[rating] * (1 if registered else 1-coeff["kc_no_registration_penalty_pct"])
        time = exp(-lambda_ * age / (kh * kb))
        return min(price, price * time * kc)
    truth = score(row[-2], .95, "B", True)
    for name, (hours, conditions, registrations) in masks.items():
        options = [candidate[-2] for candidate in cohorts[row[:3]] if candidate[-1] <= factory_year]
        values = [score(*p) for p in product(options, hours, conditions, registrations)]
        low, high = min(values), max(values)
        assert low <= truth <= high
        width = (high - low) / ((high + low) / 2)
        for i, limit in enumerate(limits):
            stats[name][i] += int(width <= limit+1e-12)
print(f"{len(rows)} catalog rows; {len(cohorts)} cohorts; {len(brands)} brands in coefficient dictionary")
for mask, counts in stats.items():
    print(mask, [(int(limit*100), n, round(n/len(rows)*100,1)) for limit,n in zip(limits,counts)])
