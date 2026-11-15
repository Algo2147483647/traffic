// Generated from Go JSON contracts by go run ./scripts/api-types. Do not hand edit.
export interface City {
  "macro_interchanges"?: Array<InterchangeNode>;
  "world_units_per_km": number;
  "interchanges"?: Array<RoadInterchange>;
  "name": string;
  "description": string;
  "driving_side": string;
  "movement_rules": RoadMovementRulesConfig;
  "bounds": RoadBounds;
  "simulation": RoadSimulationConfig;
  "intersections": Array<RoadIntersection>;
  "ways": Array<RoadWay>;
  "edges": Array<RoadEdge>;
}

export interface DemandConfig {
  "trips_per_minute": number;
  "metro_share": number;
  "taxi_share": number;
  "seed": number;
}

export interface DemandModeSummary {
  "generated": number;
  "queued": number;
  "active": number;
  "completed": number;
  "cancelled": number;
}

export interface DemandSummary {
  "modes": Record<string, DemandModeSummary>;
  "conserved": boolean;
  "errors": Array<string>;
  "config": DemandConfig;
  "generated": number;
  "queued": number;
  "active": number;
  "completed": number;
  "cancelled": number;
  "car_trips": number;
  "metro_trips": number;
  "taxi_trips": number;
}

export interface FoundationEntityRef {
  "run": number;
  "id": number;
}

export interface FoundationStamp {
  "run": number;
  "world": number;
  "topology": number;
}

export interface GeometryPoint {
  "x": number;
  "y": number;
}

export interface GeometryPose {
  "position": GeometryPoint;
  "heading": number;
}

export interface InterchangeNode {
  "id": string;
  "ports": Array<InterchangePort>;
  "internal_ways": Array<string>;
}

export interface InterchangePort {
  "node_id": string;
  "incoming": Array<string>;
  "outgoing": Array<string>;
}

export interface IntersectionsMovementPressure {
  "from_way_id": string;
  "to_way_id": string;
  "from_lanes": Array<number>;
  "to_lanes": Array<number>;
  "lane_connections": Array<RoadLaneConnectionConfig>;
  "incoming_queue": number;
  "outgoing_queue": number;
  "pressure": number;
}

export interface IntersectionsPhasePressure {
  "phase_index": number;
  "phase_id": string;
  "incoming_queue": number;
  "outgoing_queue": number;
  "pressure": number;
  "movements": Array<IntersectionsMovementPressure>;
}

export interface MetroBlock {
  "id": string;
  "from": number;
  "to": number;
  "start_m": number;
  "end_m": number;
  "reserved_by": number;
  "occupied_by": number;
}

export interface MetroConfig {
  "service"?: MetroServiceConfig;
  "line_elevations_m"?: Record<string, number>;
  "line_layers"?: Record<string, number>;
  "cars": number;
  "max_speed_kmh": number;
  "enabled": boolean;
  "headway_s": number;
  "passengers_per_minute": number;
  "capacity": number;
  "dwell_s": number;
  "seed": number;
}

export interface MetroJourneyStep {
  "line": number;
  "from": number;
  "to": number;
  "estimated_seconds": number;
}

export interface MetroLineView {
  "elevation_m": number;
  "stop_chainages"?: Array<Array<number>>;
  "reverse_track"?: Array<GeometryPoint>;
  "circular": boolean;
  "layer": number;
  "track": Array<GeometryPoint>;
  "tunnel_inner_diameter_m": number;
  "tunnel_outer_diameter_m": number;
  "id": number;
  "name": string;
  "stations": Array<number>;
}

export interface MetroNetworkJourney {
  "state": string;
  "id": number;
  "journey_id": number;
  "origin": number;
  "destination": number;
  "plan": Array<MetroJourneyStep>;
  "step": number;
  "ready_at": number;
  "leg_active": boolean;
}

export interface MetroPlatformDetail {
  "elevation_m": number;
  "station": number;
  "line": number;
  "layer": number;
  "directions": Array<MetroPlatformDirection>;
}

export interface MetroPlatformDirection {
  "direction": number;
  "toward": number;
  "waiting": number;
  "train": number;
  "eta_seconds": number;
  "status": string;
}

export interface MetroRoute {
  "station": number;
  "owner": number;
}

export interface MetroServiceConfig {
  "algorithm": string;
  "target_headway_s": number;
}

export interface MetroServicePlan {
  "line": number;
  "name": string;
  "fleet": number;
  "headway_s": number;
  "requested_headway_s": number;
  "cycle_s": number;
  "minimum_headway_s": number;
  "circulation_limit": number;
}

export interface MetroServiceTrip {
  "id": number;
  "direction": number;
  "origin": number;
  "planned_s": number;
  "actual_s": number;
  "finished_s": number;
  "train": number;
  "state": string;
}

export interface MetroServiceView {
  "plan": MetroServicePlan;
  "active": number;
  "depot": number;
  "retiring": number;
  "withdrawn": number;
  "departed": number;
  "completed": number;
  "trips": Array<MetroServiceTrip>;
  "depots": Array<number>;
}

export interface MetroStationDetail {
  "id": string;
  "transferring": number;
  "platforms": Array<MetroPlatformDetail>;
  "passenger_ids": Array<number>;
}

export interface MetroStationView {
  "elevation_m": number;
  "transfer"?: string;
  "layer": number;
  "id": number;
  "name": string;
  "position": GeometryPoint;
  "waiting": number;
}

export interface MetroTrainView {
  "elevation_m": number;
  "layer": number;
  "car_poses"?: Array<GeometryPose>;
  "car_length_m": number;
  "car_width_m": number;
  "line": number;
  "cars": Array<number>;
  "car_capacities": Array<number>;
  "heading": number;
  "id": number;
  "station": number;
  "next": number;
  "direction": number;
  "state": string;
  "position": GeometryPoint;
  "speed_mps": number;
  "length_m": number;
  "capacity": number;
  "passengers": number;
  "departure_s": number;
  "delay_s": number;
  "distance_m": number;
  "authority_m": number;
}

export interface MetroTransferSite {
  "id": string;
  "name": string;
  "stations": Array<number>;
  "position": GeometryPoint;
  "walk_seconds": number;
}

export interface MetroView {
  "services"?: Array<MetroServiceView>;
  "service_planners"?: Array<string>;
  "station_details"?: Array<MetroStationDetail>;
  "highest_layer": number;
  "transferring": number;
  "transfers_completed": number;
  "interchanges"?: Array<MetroTransferSite>;
  "journeys"?: Array<MetroNetworkJourney>;
  "crossing_owner": number;
  "lines": Array<MetroLineView>;
  "layer": number;
  "config": MetroConfig;
  "time": number;
  "stations": Array<MetroStationView>;
  "trains": Array<MetroTrainView>;
  "blocks": Array<MetroBlock>;
  "routes": Array<MetroRoute>;
  "generated": number;
  "waiting": number;
  "onboard": number;
  "completed": number;
  "departures": number;
}

export interface RoadApproach {
  "way_id": string;
  "direction": string;
  "lanes": number;
  "transform": Array<Array<number>>;
  "sub_segments": Array<RoadSubSegment>;
}

export interface RoadBounds {
  "min_x": number;
  "min_y": number;
  "max_x": number;
  "max_y": number;
}

export interface RoadEdge {
  "way_id": string;
  "from": string;
  "to": string;
  "weight": number;
}

export interface RoadInterchange {
  "id": string;
  "name": string;
  "type": string;
  "center": RoadVec2;
  "routes": Array<RoadInterchangeRoute>;
}

export interface RoadInterchangeRoute {
  "origin": string;
  "destination": string;
  "turn": string;
  "way_ids": Array<string>;
}

export interface RoadIntersection {
  "layer": number;
  "elevation": number;
  "interchange"?: string;
  "id": string;
  "name": string;
  "position": RoadVec2;
  "transform": Array<Array<number>>;
  "approaches": Array<RoadApproach>;
  "traffic_control": RoadTrafficControl;
}

export interface RoadLaneConnectionConfig {
  "from_lane": number;
  "to_lane": number;
}

export interface RoadMovementRulesConfig {
  "through_tolerance_degrees": number;
  "u_turn_tolerance_degrees": number;
}

export interface RoadSchedulingAlgorithmChoice {
  "id": string;
  "parameters": Record<string, number>;
}

export interface RoadSchedulingAlgorithmConfig {
  "demand": RoadSchedulingAlgorithmChoice;
  "supply": RoadSchedulingAlgorithmChoice;
  "motion": RoadSchedulingAlgorithmChoice;
  "metro": RoadSchedulingAlgorithmChoice;
  "route": RoadSchedulingAlgorithmChoice;
  "signal": RoadSchedulingAlgorithmChoice;
}

export interface RoadSchedulingAlgorithmView {
  "config": RoadSchedulingAlgorithmConfig;
  "pending": boolean;
  "applied": number;
  "fallbacks": number;
  "last_error": string;
}

export interface RoadSchedulingControlPerformance {
  "algorithm": string;
  "vehicle_delay_seconds": number;
  "signal_delay_seconds": number;
  "mean_signal_delay_seconds": number;
  "signal_stops": number;
  "signal_releases": number;
  "completed_trips": number;
  "throughput_per_minute": number;
  "current_signal_queue": number;
  "maximum_signal_queue": number;
}

export interface RoadSchedulingLaneChange {
  "from_lane": number;
  "to_lane": number;
  "progress": number;
  "reason": string;
}

export interface RoadSchedulingLoadState {
  "population": number;
  "spawn_rate_per_second": number;
  "max_population": number;
}

export interface RoadSchedulingMovement {
  "from_way_id": string;
  "to_way_id": string;
  "from_lanes": Array<number>;
  "to_lanes": Array<number>;
  "lane_connections": Array<RoadLaneConnectionConfig>;
  "turn": string;
  "demand_score": number;
}

export interface RoadSchedulingPerson {
  "journey_id"?: number;
  "entity": FoundationEntityRef;
  "structure": string;
  "elevation": number;
  "id": number;
  "state": string;
  "position": RoadVec2;
  "speed": number;
  "heading_degrees": number;
  "way_id"?: string;
  "lane": number;
  "lane_position": number;
  "lane_change"?: RoadSchedulingLaneChange;
  "along": number;
  "transform": Array<Array<number>>;
  "reference": RoadSchedulingReference;
  "waiting_for"?: number;
  "waiting_signal"?: string;
  "waiting_reason"?: string;
  "origin": RoadVec2;
  "destination": RoadVec2;
  "route": Array<string>;
  "route_entry_lanes"?: Array<number>;
  "route_exit_lanes"?: Array<number>;
  "route_index": number;
  "trip": number;
}

export interface RoadSchedulingReference {
  "intersection_id": string;
  "intersection_transform": Array<Array<number>>;
  "local_transform": Array<Array<number>>;
}

export interface RoadSchedulingSignalState {
  "intersection_id": string;
  "phase_id": string;
  "phase_name": string;
  "stage": string;
  "phase_index": number;
  "remaining_seconds": number;
  "cycle_seconds": number;
  "phase_elapsed_seconds": number;
  "next_phase_id": string;
  "next_phase_name": string;
  "allowed_incoming_way_ids": Array<string>;
  "allowed_movements": Array<RoadSchedulingMovement>;
  "controller": string;
  "decision": string;
  "base_green_seconds": number;
  "target_green_seconds": number;
  "min_green_seconds": number;
  "max_green_seconds": number;
  "queue_pressure": number;
  "competing_pressure": number;
  "incoming_queue": number;
  "outgoing_queue": number;
  "phase_pressures": Array<IntersectionsPhasePressure>;
  "extension_seconds": number;
}

export interface RoadSchedulingSpacingState {
  "safe_gap": number;
  "vehicle_size": number;
}

export interface RoadSchedulingStats {
  "walking": number;
  "driving": number;
  "waiting": number;
  "arrived": number;
  "average_speed": number;
}

export interface RoadSchedulingTaxiVehicle extends TaxiVehicle {

}

export interface RoadSchedulingTaxiView {
  "config": TaxiConfig;
  "stops": Array<TaxiStop>;
  "orders": Array<TaxiOrder>;
  "vehicles": Array<RoadSchedulingTaxiVehicle>;
  "created": number;
  "completed": number;
  "cancelled": number;
  "picked": number;
  "mean_wait_s": number;
  "failures": number;
  "last_error": string;
  "solving": boolean;
  "solver_ms": number;
  "candidates": number;
}

export interface RoadSegment {
  "id": string;
  "way_id": string;
  "index": number;
  "start": RoadVec2;
  "end": RoadVec2;
  "length": number;
  "transform": Array<Array<number>>;
  "sub_segments": Array<RoadSubSegment>;
}

export interface RoadSimulationConfig {
  "people": number;
  "spawn_rate_per_second": number;
  "max_people": number;
  "tick_ms": number;
  "walk_speed": number;
  "drive_speed": number;
  "safe_gap": number;
  "spawn_margin": number;
  "seed": number;
  "snapshot_fps": number;
  "vehicle_size": number;
  "intersection_radius": number;
  "arrival_dwell_seconds": number;
}

export interface RoadSubSegment {
  "id": string;
  "lane": number;
  "start": RoadVec2;
  "end": RoadVec2;
  "length": number;
  "transform": Array<Array<number>>;
}

export interface RoadTrafficControl {
  "mode"?: string;
  "offset_seconds": number;
  "cycle_seconds": number;
  "phases": Array<RoadTrafficPhaseConfig>;
  "generated": boolean;
  "planning_basis": string;
  "timing_method": string;
  "adaptive": boolean;
  "lost_time_seconds": number;
  "design_volume_ratio": number;
  "max_red_seconds": number;
}

export interface RoadTrafficMovementConfig {
  "from_way_id": string;
  "to_way_id": string;
  "from_lanes": Array<number>;
  "to_lanes": Array<number>;
  "lane_connections": Array<RoadLaneConnectionConfig>;
  "turn": string;
  "demand_score"?: number;
}

export interface RoadTrafficPhaseConfig {
  "id": string;
  "name": string;
  "duration_seconds": number;
  "min_green_seconds"?: number;
  "max_green_seconds"?: number;
  "amber_seconds"?: number;
  "all_red_seconds"?: number;
  "demand_score"?: number;
  "critical_flow_ratio"?: number;
  "allowed_incoming_way_ids"?: Array<string>;
  "allowed_movements"?: Array<RoadTrafficMovementConfig>;
}

export interface RoadVec2 {
  "x": number;
  "y": number;
}

export interface RoadWay {
  "trip_access": boolean;
  "role"?: string;
  "geometry"?: string;
  "structure": string;
  "start_layer": number;
  "end_layer": number;
  "start_elevation": number;
  "end_elevation": number;
  "id": string;
  "name": string;
  "from": string;
  "to": string;
  "lanes": number;
  "lane_width": number;
  "carriageway_offset": number;
  "speed_limit": number;
  "road_class": string;
  "demand_weight": number;
  "weight": number;
  "length": number;
  "points": Array<RoadVec2>;
  "segments": Array<RoadSegment>;
}

export interface Snapshot {
  "metro": MetroView;
  "demand"?: DemandSummary;
  "algorithms": RoadSchedulingAlgorithmView;
  "walk_speed_mps": number;
  "kernel": FoundationStamp;
  "mode": string;
  "taxi"?: RoadSchedulingTaxiView;
  "city": string;
  "time": number;
  "tick": number;
  "paused": boolean;
  "time_scale": number;
  "people": Array<RoadSchedulingPerson>;
  "stats": RoadSchedulingStats;
  "load": RoadSchedulingLoadState;
  "spacing": RoadSchedulingSpacingState;
  "signals": Array<RoadSchedulingSignalState>;
  "control_performance": RoadSchedulingControlPerformance;
}

export interface TaxiConfig {
  "fleet": number;
  "orders_per_minute": number;
  "algorithm": string;
  "seed": number;
  "max_wait_s": number;
  "dwell_s": number;
  "dispatch_interval_s": number;
  "parameters"?: Record<string, number>;
}

export interface TaxiOrder {
  "journey_id"?: number;
  "id": number;
  "pickup": string;
  "destination": string;
  "state": string;
  "vehicle_id": number;
  "created_s": number;
  "assigned_s": number;
  "picked_s": number;
  "completed_s": number;
  "pickup_eta_s": number;
}

export interface TaxiStop {
  "id": string;
  "way_id": string;
  "along": number;
  "position": RoadVec2;
}

export interface TaxiVehicle {
  "retiring": boolean;
  "id": number;
  "state": string;
  "order_id": number;
  "completed": number;
  "income": number;
  "occupied_s": number;
  "empty_distance_m": number;
  "occupied_distance_m": number;
}

