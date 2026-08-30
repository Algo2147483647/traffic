export type Point = { x: number; y: number };
export type Matrix = number[][];
export type SimulationSettings = {
  people: number; spawn_rate_per_second: number; max_people: number;
  vehicle_size: number; intersection_radius: number; safe_gap: number;
};
export type Movement = {
  from_way_id: string; to_way_id: string; from_lanes: number[]; to_lanes: number[];
  lane_connections: { from_lane: number; to_lane: number }[];
  turn: "left" | "right" | "through" | "u_turn"; demand_score: number;
};
export type TrafficPhase = {
  id: string; name: string; duration_seconds: number; amber_seconds: number;
  all_red_seconds: number; demand_score: number; min_green_seconds: number;
  max_green_seconds: number; critical_flow_ratio: number; allowed_incoming_way_ids: string[];
  allowed_movements: Movement[];
};
export type TrafficControl = {
  offset_seconds: number; cycle_seconds: number; phases: TrafficPhase[];
  generated: boolean; planning_basis: string; timing_method: string; adaptive: boolean;
  lost_time_seconds: number; design_volume_ratio: number; max_red_seconds: number;
};
export type Intersection = { id: string; name: string; position: Point; traffic_control: TrafficControl; approaches?: { sub_segments: { transform: Matrix }[] }[] };
export type Way = {
  id: string; name?: string; from: string; to: string; lanes: number; lane_width: number;
  carriageway_offset: number; speed_limit: number; road_class: "local" | "collector" | "ramp" | "arterial";
  demand_weight: number; length?: number; points: Point[];
};
export type City = {
  name: string; description: string;
  driving_side: "right" | "left";
  movement_rules: { through_tolerance_degrees: number; u_turn_tolerance_degrees: number };
  bounds: { min_x: number; min_y: number; max_x: number; max_y: number };
  simulation: SimulationSettings; intersections: Intersection[]; ways: Way[]; edges: { weight: number }[];
};
export type Signal = {
  intersection_id: string; phase_id: string; phase_name: string; phase_index: number;
  stage: "green" | "amber" | "all_red"; remaining_seconds: number; phase_elapsed_seconds?: number;
  cycle_seconds: number; next_phase_id?: string; next_phase_name?: string;
  allowed_incoming_way_ids: string[]; allowed_movements: Movement[];
  controller: string; decision: string; base_green_seconds: number; target_green_seconds: number;
  min_green_seconds: number; max_green_seconds: number; queue_pressure: number;
  competing_pressure: number; incoming_queue?: number; outgoing_queue?: number;
  phase_pressures?: { phase_index: number; phase_id: string; incoming_queue: number; outgoing_queue: number; pressure: number;
    movements?: { from_way_id: string; to_way_id: string; from_lanes: number[]; to_lanes: number[]; lane_connections: {from_lane:number;to_lane:number}[]; incoming_queue: number; outgoing_queue: number; pressure: number }[] }[];
  extension_seconds: number;
};
export type Person = {
  id: number; state: string; position: Point; speed: number; heading_degrees: number;
  way_id?: string; lane: number; lane_position: number;
  lane_change?: { from_lane: number; to_lane: number; progress: number; reason: string };
  origin: Point; destination: Point; route: string[];
  route_index: number; trip: number; reference: { intersection_id: string; local_transform: Matrix };
};
export type Snapshot = {
  time: number; paused: boolean; time_scale: number; people: Person[]; signals: Signal[];
  load: { population: number; spawn_rate_per_second: number; max_population: number };
  spacing: { safe_gap: number; vehicle_size: number };
  stats: { walking: number; driving: number; waiting: number; arrived: number; average_speed: number };
  control_performance: {
    algorithm: string; vehicle_delay_seconds: number; signal_delay_seconds: number;
    mean_signal_delay_seconds: number; signal_stops: number; signal_releases: number;
    completed_trips: number; throughput_per_minute: number; current_signal_queue: number;
    maximum_signal_queue: number;
  };
};
export type Selection = { type: "person" | "intersection"; id: number | string } | null;

const baseIntersections = [
  ["AN","North Interchange",1800,260],["AW","West Arterial",850,1100],["AC","Central Exchange",1800,1100],["AE","East Arterial",2750,1100],["AS","South Interchange",1800,1940],
  ["CNW","Northwest Collector",900,600],["CNE","Northeast Collector",2700,600],["CSW","Southwest Collector",900,1600],["CSE","Southeast Collector",2700,1600],
  ["NW1","Cedar Homes",180,280],["NW2","Museum Row",260,720],["NWH","Cedar Merge",560,520],
  ["NE1","Aurora Homes",3420,280],["NE2","Harbor Row",3340,720],["NEH","Aurora Merge",3040,520],
  ["SW1","Garden Homes",180,1900],["SW2","Ember Row",300,1450],["SWH","Garden Merge",570,1680],
  ["SE1","Foundry Homes",3420,1900],["SE2","Quay Row",3300,1450],["SEH","Foundry Merge",3030,1680],
] as const;
const nodes: Record<string, Point> = Object.fromEntries(baseIntersections.map(([id,,x,y]) => [id,{x,y}]));
type RoadClass = Way["road_class"];
type PhysicalRoad = { a: string; b: string; lanes: number; roadClass: RoadClass; via?: Point[]; demand?: number };
const roads: PhysicalRoad[] = [
  {a:"AW",b:"AC",lanes:4,roadClass:"arterial",demand:2.3},{a:"AC",b:"AE",lanes:4,roadClass:"arterial",demand:2.4},{a:"AN",b:"AC",lanes:4,roadClass:"arterial",demand:2.1},{a:"AC",b:"AS",lanes:4,roadClass:"arterial",demand:2.0},
  {a:"CNW",b:"AW",lanes:2,roadClass:"ramp",via:[{x:760,y:820}],demand:1.2},{a:"CNW",b:"AN",lanes:2,roadClass:"ramp",via:[{x:1250,y:420}],demand:1.15},
  {a:"CNE",b:"AE",lanes:2,roadClass:"ramp",via:[{x:2840,y:820}],demand:1.2},{a:"CNE",b:"AN",lanes:2,roadClass:"ramp",via:[{x:2350,y:420}],demand:1.15},
  {a:"CSW",b:"AW",lanes:2,roadClass:"ramp",via:[{x:760,y:1380}],demand:1.2},{a:"CSW",b:"AS",lanes:2,roadClass:"ramp",via:[{x:1250,y:1780}],demand:1.15},
  {a:"CSE",b:"AE",lanes:2,roadClass:"ramp",via:[{x:2840,y:1380}],demand:1.2},{a:"CSE",b:"AS",lanes:2,roadClass:"ramp",via:[{x:2350,y:1780}],demand:1.15},
  {a:"NW1",b:"NWH",lanes:1,roadClass:"local"},{a:"NW2",b:"NWH",lanes:1,roadClass:"local"},{a:"NWH",b:"CNW",lanes:2,roadClass:"collector",demand:1},
  {a:"NE1",b:"NEH",lanes:1,roadClass:"local"},{a:"NE2",b:"NEH",lanes:1,roadClass:"local"},{a:"NEH",b:"CNE",lanes:2,roadClass:"collector",demand:1},
  {a:"SW1",b:"SWH",lanes:1,roadClass:"local"},{a:"SW2",b:"SWH",lanes:1,roadClass:"local"},{a:"SWH",b:"CSW",lanes:2,roadClass:"collector",demand:1},
  {a:"SE1",b:"SEH",lanes:1,roadClass:"local"},{a:"SE2",b:"SEH",lanes:1,roadClass:"local"},{a:"SEH",b:"CSE",lanes:2,roadClass:"collector",demand:1},
];
const classSpeed: Record<RoadClass,number> = {local:55,collector:78,ramp:92,arterial:120};
const classWidth: Record<RoadClass,number> = {local:7,collector:7.5,ramp:8,arterial:8};
const movementRules = {through_tolerance_degrees:40,u_turn_tolerance_degrees:35};
const allWays: Way[] = roads.flatMap((road) => {
  const points = [nodes[road.a],...(road.via || []),nodes[road.b]];
  const make = (from:string,to:string,path:Point[]): Way => ({
    id:`${from}-${to}`,name:`${from} / ${to}`,from,to,lanes:road.lanes,lane_width:classWidth[road.roadClass],
    carriageway_offset:road.lanes*classWidth[road.roadClass]/2+1.5,speed_limit:classSpeed[road.roadClass],
    road_class:road.roadClass,demand_weight:road.demand || (road.roadClass === "local" ? .55 : 1),points:path,
  });
  return [make(road.a,road.b,points),make(road.b,road.a,[...points].reverse())];
});

function turnType(incoming: Way, outgoing: Way): Movement["turn"] {
  const a = sampleWay(incoming, Math.max(0,wayLength(incoming)-1)).tangent;
  const b = sampleWay(outgoing, Math.min(1,wayLength(outgoing))).tangent;
  const dot = a.x*b.x+a.y*b.y, cross = a.x*b.y-a.y*b.x, angle=Math.atan2(cross,dot);
  if (Math.PI-Math.abs(angle) <= movementRules.u_turn_tolerance_degrees*Math.PI/180) return "u_turn";
  if (Math.abs(angle) <= movementRules.through_tolerance_degrees*Math.PI/180) return "through";
  return cross < 0 ? "left" : "right";
}
function lanesFor(way: Way, turn: Movement["turn"]) {
  if (way.lanes === 1) return [0];
  if (turn === "left" || turn === "u_turn") return [0];
  if (turn === "right") return [way.lanes-1];
  return Array.from({length:way.lanes},(_,lane)=>lane);
}
function exitLanes(way: Way) {
  return Array.from({length:way.lanes},(_,lane)=>lane);
}
function connectLanes(fromLanes:number[],toLanes:number[]) {
  const from=[...fromLanes].sort((a,b)=>a-b),to=[...toLanes].sort((a,b)=>a-b),connections:{from_lane:number;to_lane:number}[]=[];
  from.forEach((fromLane,fromIndex)=>{
    const start=Math.round(fromIndex*to.length/from.length),end=Math.round((fromIndex+1)*to.length/from.length);
    if(start===end){const target=from.length>1&&to.length>1?Math.round(fromIndex*(to.length-1)/(from.length-1)):0;connections.push({from_lane:fromLane,to_lane:to[target]});return;}
    for(let target=start;target<end;target++)connections.push({from_lane:fromLane,to_lane:to[target]});
  });
  return connections;
}
function movement(from: Way,to: Way): Movement {
  const turn=turnType(from,to),factor=turn==="left"?.2:turn==="right"?.15:turn==="u_turn"?.03:.65,fromLanes=lanesFor(from,turn),toLanes=exitLanes(to);
  return {from_way_id:from.id,to_way_id:to.id,from_lanes:fromLanes,to_lanes:toLanes,lane_connections:connectLanes(fromLanes,toLanes),turn,demand_score:from.demand_weight*factor};
}
function tangentAtEnd(way:Way){return sampleWay(way,Math.max(0,wayLength(way)-1)).tangent;}
function compatible(movements:Movement[],incomingByID:Record<string,Way>){
  for(let first=0;first<movements.length;first++)for(let second=first+1;second<movements.length;second++){
    const a=movements[first],b=movements[second];if(a.from_way_id===b.from_way_id)continue;if(a.to_way_id===b.to_way_id)return false;
    const ta=tangentAtEnd(incomingByID[a.from_way_id]),tb=tangentAtEnd(incomingByID[b.from_way_id]),dot=ta.x*tb.x+ta.y*tb.y;
    if(dot>-.7||(a.turn==="left")!==(b.turn==="left"))return false;
  }
  return true;
}
function makeControl(id:string,index:number): TrafficControl {
  const incoming = allWays.filter((way)=>way.to===id), outgoing = allWays.filter((way)=>way.from===id);
  const sorted=[...incoming].sort((a,b)=>a.id.localeCompare(b.id)),used=new Set<string>(),groups:Way[][]=[];
  sorted.forEach((way)=>{if(used.has(way.id))return;const tangent=tangentAtEnd(way);let best:Way|undefined,bestDot=-.7;sorted.forEach((candidate)=>{if(candidate.id===way.id||used.has(candidate.id))return;const other=tangentAtEnd(candidate),dot=tangent.x*other.x+tangent.y*other.y;if(dot<bestDot){bestDot=dot;best=candidate;}});const group=[way];used.add(way.id);if(best){group.push(best);used.add(best.id);}groups.push(group);});
  type Draft={id:string;name:string;minimum:number;amber:number;red:number;movements:Movement[]};const drafts:Draft[]=[];
  const rank:Record<RoadClass,number>={local:0,ramp:1,collector:2,arterial:3};
  groups.forEach((group,groupIndex)=>{
    const roadClass=group.reduce((best,way)=>rank[way.road_class]>rank[best]?way.road_class:best,"local" as RoadClass),label=roadClass[0].toUpperCase()+roadClass.slice(1);
    const main:Draft={id:`axis-${groupIndex+1}-main`,name:`${label} through + right`,minimum:roadClass==="arterial"?16:roadClass==="collector"?11:roadClass==="ramp"?9:7,amber:3,red:1.5,movements:[]};
    const left:Draft={id:`axis-${groupIndex+1}-left`,name:`${label} protected left`,minimum:roadClass==="arterial"?9:7,amber:3,red:1.5,movements:[]};
    const turnaround:Draft={id:`axis-${groupIndex+1}-turnaround`,name:`${label} turnaround`,minimum:6,amber:2,red:1,movements:[]};
    group.forEach((from)=>outgoing.forEach((to)=>{const item=movement(from,to);if(item.turn==="left")left.movements.push(item);else if(item.turn==="u_turn"){if(outgoing.length===1)turnaround.movements.push(item);}else main.movements.push(item);}));
    [main,left].forEach((draft)=>{if(!draft.movements.length)return;const incomingByID=Object.fromEntries(incoming.map((way)=>[way.id,way]));if(compatible(draft.movements,incomingByID))drafts.push(draft);else{const ids=[...new Set(draft.movements.map((item)=>item.from_way_id))].sort();ids.forEach((fromID,part)=>drafts.push({...draft,id:`${draft.id}-${part+1}`,name:`${draft.name} · ${fromID}`,movements:draft.movements.filter((item)=>item.from_way_id===fromID)}));}});
    if(turnaround.movements.length)drafts.push(turnaround);
  });
  const phases:TrafficPhase[]=drafts.map((draft)=>{const demand=draft.movements.reduce((sum,item)=>sum+item.demand_score,0),protectedLanes=new Set(draft.movements.flatMap((item)=>item.from_lanes.map((lane)=>`${item.from_way_id}:${lane}`))).size;return{id:draft.id,name:draft.name,duration_seconds:draft.minimum,min_green_seconds:draft.minimum,max_green_seconds:draft.minimum,amber_seconds:draft.amber,all_red_seconds:draft.red,demand_score:demand,critical_flow_ratio:demand/Math.max(2,protectedLanes*2),allowed_incoming_way_ids:[...new Set(draft.movements.map((item)=>item.from_way_id))].sort(),allowed_movements:draft.movements};});
  const lostTime=phases.reduce((sum,phase)=>sum+phase.amber_seconds+phase.all_red_seconds,0),minimumTotal=phases.reduce((sum,phase)=>sum+phase.min_green_seconds,0),totalRatio=phases.reduce((sum,phase)=>sum+phase.critical_flow_ratio,0);
  const webster=(1.5*lostTime+5)/Math.max(.08,1-Math.min(.92,totalRatio)),target=Math.max(55,Math.min(120,webster),lostTime+minimumTotal),extra=Math.max(0,target-lostTime-minimumTotal);
  phases.forEach((phase)=>{const share=totalRatio?phase.critical_flow_ratio/totalRatio:1/phases.length;phase.duration_seconds=Math.round((phase.min_green_seconds+extra*share)*2)/2;phase.max_green_seconds=Math.round(Math.min(phase.duration_seconds+20,Math.max(phase.min_green_seconds+4,phase.duration_seconds*1.55))*2)/2;});
  return {offset_seconds:index*2.3,cycle_seconds:phases.reduce((sum,phase)=>sum+phase.duration_seconds+phase.amber_seconds+phase.all_red_seconds,0),phases,generated:true,planning_basis:"Conflict-free protected movements over complete lane connectors; Webster green bounds with differential-queue max-pressure scheduling",timing_method:"Webster green bounds + differential-queue max-pressure scheduling",adaptive:true,lost_time_seconds:lostTime,design_volume_ratio:totalRatio,max_red_seconds:0};
}
const intersections: Intersection[] = baseIntersections.map(([id,name,x,y],index)=>({id,name,position:{x,y},approaches:[],traffic_control:makeControl(id,index)}));

export const demoCity: City = {
  name:"Meridian Hierarchy",
  description:"Neighborhood streets merge into collectors and ramps before reaching the long cross-city arterial spine.",
  driving_side:"right",
  movement_rules:movementRules,
  bounds:{min_x:0,min_y:0,max_x:3600,max_y:2200},
  simulation:{people:320,spawn_rate_per_second:2.5,max_people:1600,vehicle_size:18,intersection_radius:58,safe_gap:26},
  intersections,ways:allWays,edges:allWays.map((way)=>({weight:way.lanes})),
};

const fract = (value:number)=>value-Math.floor(value);
const random = (seed:number)=>fract(Math.sin(seed*12.9898+78.233)*43758.5453);
export function wayLength(way:Way) { return way.length || way.points.slice(1).reduce((sum,point,index)=>sum+Math.hypot(point.x-way.points[index].x,point.y-way.points[index].y),0); }
export function sampleWay(way:Way,along:number,lane=0,extraOffset=0) {
  const total=wayLength(way),target=Math.max(0,Math.min(total,along)); let covered=0;
  for(let index=0;index<way.points.length-1;index++){
    const a=way.points[index],b=way.points[index+1],dx=b.x-a.x,dy=b.y-a.y,length=Math.hypot(dx,dy);
    if(covered+length>=target||index===way.points.length-2){const part=length?(target-covered)/length:0,tx=dx/length,ty=dy/length,offset=(way.carriageway_offset||0)+(lane-(way.lanes-1)/2)*way.lane_width+extraOffset;return {position:{x:a.x+dx*part-ty*offset,y:a.y+dy*part+tx*offset},tangent:{x:tx,y:ty}};}
    covered+=length;
  }
  return {position:way.points[0],tangent:{x:1,y:0}};
}
function matrixFor(position:Point,heading:number,reference:Point):Matrix {const angle=heading*Math.PI/180,c=Math.cos(angle),s=Math.sin(angle);return [[c,-s,position.x-reference.x],[s,c,position.y-reference.y],[0,0,1]];}
function demoSignals(city:City,time:number):Signal[]{
  return city.intersections.map((intersection)=>{
    const control=intersection.traffic_control;
    const pressures=control.phases.map((phase,index)=>Math.max(.2,phase.demand_score*(2.5+2*Math.sin(time*.045+index*1.71+intersection.position.x*.003))));
    const targets=control.phases.map((phase,index)=>{const competing=Math.max(0,...pressures.filter((_,candidate)=>candidate!==index)),raw=pressures[index]<competing*.35?phase.min_green_seconds:phase.duration_seconds+(pressures[index]-competing)*1.8;return Math.round(Math.max(phase.min_green_seconds,Math.min(phase.max_green_seconds,raw))*2)/2;});
    const adaptiveCycle=control.phases.reduce((sum,phase,index)=>sum+targets[index]+phase.amber_seconds+phase.all_red_seconds,0),position=(time+control.offset_seconds)%adaptiveCycle;let phaseIndex=0,elapsed=0,stage:Signal["stage"]="green",remaining=0;
    for(let index=0;index<control.phases.length;index++){
      const phase=control.phases[index],greenEnd=elapsed+targets[index],amberEnd=greenEnd+phase.amber_seconds,redEnd=amberEnd+phase.all_red_seconds;
      if(position<greenEnd){phaseIndex=index;stage="green";remaining=greenEnd-position;break} if(position<amberEnd){phaseIndex=index;stage="amber";remaining=amberEnd-position;break} if(position<redEnd){phaseIndex=index;stage="all_red";remaining=redEnd-position;break} elapsed=redEnd;
    }
    const phase=control.phases[phaseIndex],next=control.phases[(phaseIndex+1)%control.phases.length],open=stage==="green",competing=Math.max(0,...pressures.filter((_,candidate)=>candidate!==phaseIndex)),target=targets[phaseIndex];
    const decision=stage!=="green"?"conflict clearance":target<phase.duration_seconds?"gap out":target>phase.duration_seconds?"queue extension":"design green";
    return {intersection_id:intersection.id,phase_id:phase.id,phase_name:phase.name,phase_index:phaseIndex,stage,remaining_seconds:remaining,phase_elapsed_seconds:position-elapsed,cycle_seconds:adaptiveCycle,next_phase_id:next.id,next_phase_name:next.name,allowed_incoming_way_ids:open?phase.allowed_incoming_way_ids:[],allowed_movements:open?phase.allowed_movements:[],controller:"adaptive max-pressure",decision,base_green_seconds:phase.duration_seconds,target_green_seconds:target,min_green_seconds:phase.min_green_seconds,max_green_seconds:phase.max_green_seconds,queue_pressure:pressures[phaseIndex],competing_pressure:competing,extension_seconds:Math.max(0,target-phase.duration_seconds)};
  });
}
function demoPeople(city:City,signals:Signal[],time:number,population:number):Person[]{
  const signalByIntersection=Object.fromEntries(signals.map((signal)=>[signal.intersection_id,signal]));
  return Array.from({length:population},(_,index)=>{
    const way=city.ways[(index*7+3)%city.ways.length],length=wayLength(way),lane=index%way.lanes,start=sampleWay(way,0,lane).position,end=sampleWay(way,length,lane).position;
    const origin={x:random(index*4+1)*3600,y:random(index*4+2)*2200};let destination={x:random(index*4+3)*3600,y:random(index*4+4)*2200};if(Math.hypot(destination.x-origin.x,destination.y-origin.y)<600)destination={x:(destination.x+1810)%3600,y:(destination.y+1110)%2200};
    const cycle=58,local=(time+index*.43)%cycle;let state="driving",progress=0,position:Point,speed=way.speed_limit;
    if(local<6){state="walking_to_lane";progress=local/6;position={x:origin.x+(start.x-origin.x)*progress,y:origin.y+(start.y-origin.y)*progress};speed=14}
    else if(local<47){progress=(local-6)/41;const green=signalByIntersection[way.to]?.allowed_incoming_way_ids.includes(way.id);if(!green&&progress>.84){progress=.84;state="waiting";speed=0}position=sampleWay(way,length*progress,lane).position}
    else if(local<55){state="walking_to_destination";progress=(local-47)/8;position={x:end.x+(destination.x-end.x)*progress,y:end.y+(destination.y-end.y)*progress};speed=14}else{state="arrived";position=destination;speed=0}
    const sampled=sampleWay(way,Math.min(length,length*progress),lane),heading=state==="walking_to_destination"?Math.atan2(destination.y-end.y,destination.x-end.x)*180/Math.PI:Math.atan2(sampled.tangent.y,sampled.tangent.x)*180/Math.PI,referenceID=progress<.5?way.from:way.to;
    return {id:index+1,state,position,speed,heading_degrees:heading,way_id:state==="driving"||state==="waiting"?way.id:"",lane,lane_position:lane,origin,destination,route:[way.id],route_index:0,trip:Math.floor((time+index*.43)/cycle)+1,reference:{intersection_id:referenceID,local_transform:matrixFor(position,heading,nodes[referenceID])}};
  });
}
export function demoSnapshot(city:City,time:number,paused:boolean,scale:number,population=city.simulation.people,spawnRate=city.simulation.spawn_rate_per_second,maxPopulation=city.simulation.max_people,safeGap=city.simulation.safe_gap):Snapshot{
  const signals=demoSignals(city,time),people=demoPeople(city,signals,time,population),stats=people.reduce((result,person)=>{result[person.state.startsWith("walking")?"walking":person.state==="turning"?"driving":person.state as "driving"|"waiting"|"arrived"]++;result.average_speed+=person.speed;return result},{walking:0,driving:0,waiting:0,arrived:0,average_speed:0});stats.average_speed=people.length?stats.average_speed/people.length:0;
  const completedTrips=people.reduce((sum,person)=>sum+Math.max(0,person.trip-1),0),releases=Math.max(1,Math.round(completedTrips*1.4)),signalDelay=time*stats.waiting*.32;
  const control_performance={algorithm:"Webster + max-pressure adaptive",vehicle_delay_seconds:signalDelay*1.8,signal_delay_seconds:signalDelay,mean_signal_delay_seconds:signalDelay/releases,signal_stops:Math.round(releases*1.15),signal_releases:releases,completed_trips:completedTrips,throughput_per_minute:time?completedTrips*60/time:0,current_signal_queue:stats.waiting,maximum_signal_queue:Math.max(stats.waiting,Math.round(people.length*.12))};
  return {time,paused,time_scale:scale,people,signals,load:{population:people.length,spawn_rate_per_second:spawnRate,max_population:maxPopulation},spacing:{safe_gap:safeGap,vehicle_size:city.simulation.vehicle_size},stats,control_performance};
}
