import json, sys
request = json.load(sys.stdin)
response = {k: request[k] for k in ("contract_version", "algorithm", "algorithm_version", "capability", "stamp", "request", "input_tick")}
signals = []
for task in request["payload"]["tasks"]:
    options = [p for p in task["phases"] if p["index"] != task["current"]] or task["phases"]
    phase = max(options, key=lambda p: (p["pressure"], -p["index"]))
    signals.append({"intersection": task["intersection"], "phase": phase["index"], "green_s": phase["min_green_s"]})
response["payload"] = {"signals": signals}
print(json.dumps(response))
