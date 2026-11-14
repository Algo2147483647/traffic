"""Contract v1 example: one immutable input, one proposal, no simulation mutation."""
import json,sys
snapshot=json.loads(sys.stdin.readline())
used_vehicles=set();used_orders=set();pairs=[]
weight=snapshot.get('parameters',{}).get('wait_weight',.25)
orders={o['id']:o for o in snapshot['orders']}
for c in sorted(snapshot['candidates'],key=lambda c:(c['pickup_eta_s']-weight*(snapshot['sim_time_s']-orders[c['order_id']]['created_s']),c['order_id'],c['vehicle_id'])):
    v,o=c['vehicle_id'],c['order_id']
    if v not in used_vehicles and o not in used_orders:
        pairs.append({'vehicle_id':v,'order_id':o});used_vehicles.add(v);used_orders.add(o)
print(json.dumps({'contract_version':1,'epoch':snapshot['epoch'],'request_id':snapshot['request_id'],'assignments':pairs}))
