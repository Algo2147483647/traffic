import json, sys
request = json.load(sys.stdin)
response = {k: request[k] for k in ("contract_version", "algorithm", "algorithm_version", "capability", "stamp", "request", "input_tick")}
# Return actual lane paths. Replace this selection with a graph search if desired.
prefer_alternative = request["parameters"].get("prefer_alternative", 1)
response["payload"] = {"routes": [{"entity": t["entity"], "steps": t["options"][-1 if prefer_alternative else 0]["steps"]} for t in request["payload"]["tasks"]]}
print(json.dumps(response))
