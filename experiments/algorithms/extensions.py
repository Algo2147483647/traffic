"""Runnable alternatives. The host validates every output before commit."""
import json, sys
r = json.load(sys.stdin)
p, cap = r["payload"], r["capability"]
result = {k: r[k] for k in ("contract_version", "algorithm", "algorithm_version", "capability", "stamp", "request", "input_tick")}
if cap == "demand":
    tasks = p["tasks"]
    for task in tasks:
        task["origin"], task["destination"] = task["destination"], task["origin"]
        task["depart_at_s"] = p["time"] + 1
    out = {"journeys": tasks}
elif cap == "supply":
    stops = p["stops"]
    orders = p["orders"]
    waiting = [o["pickup"] for o in orders if o["state"] == "Waiting"]
    targets = waiting or [s["id"] for s in stops]
    out = {"targets": [{"vehicle": v, "stop": targets[i % len(targets)]} for i, v in enumerate(p["vehicles"])]}
elif cap == "motion":
    tasks = p["tasks"]
    factor = max(0.1, min(1, r.get("parameters", {}).get("speed_factor", 0.9)))
    for task in tasks:
        task["speed_mps"] *= factor
        task["merge_priority"] = 0.1 if task["entity"] % 2 == 0 else 0
    out = {"motions": tasks}
elif cap == "metro":
    out = {"trains": [{"train": t["id"], "speed_mps": p["config"]["max_speed_kmh"] / 3.6 * 0.9, "depart_after_s": p["time"] + (2 if t["passengers"] == 0 else 0)} for t in p["trains"]]}
elif cap == "geometry":
    # Mirror the complete road geometry about the vertical city centre.
    axis = p["bounds"]["min_x"] + p["bounds"]["max_x"]
    for node in p["intersections"]: node["position"]["x"] = axis - node["position"]["x"]
    for way in p["ways"]:
        for point in way.get("points", []): point["x"] = axis - point["x"]
    for interchange in p.get("interchanges", []): interchange["center"]["x"] = axis - interchange["center"]["x"]
    out = p
elif cap == "phases":
    # Select a different native phase-generation tolerance, then recompute plans.
    for node in p["intersections"]:
        control = node.get("traffic_control", {})
        phases = control.get("phases", [])
        if phases: phases.reverse()
        control["offset_seconds"] = control.get("offset_seconds", 0) + 5
    out = p
else:
    raise ValueError("unsupported capability")
result["payload"] = out
print(json.dumps(result))
