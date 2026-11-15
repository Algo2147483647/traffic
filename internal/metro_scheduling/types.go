package metroscheduling

import (
	track "traffic/internal/infrastructure/metro"
	ops "traffic/internal/operations/metro"
)

// Shared data is owned by infrastructure or operations, never by the executor.
type Layout = track.Layout
type LineSpec = track.LineSpec
type Point = track.Point
type CarPose = track.CarPose
type TransferSpec = track.TransferSpec
type Config = ops.Config
type ServiceConfig = ops.ServiceConfig

var DefaultConfig = ops.DefaultConfig

type JourneyStep = ops.JourneyStep
type NetworkJourney = ops.NetworkJourney

type StationView = ops.StationView
type Passenger = ops.Passenger
type TrainView = ops.TrainView
type Block = ops.Block
type Route = ops.Route
type LineView = ops.LineView
type View = ops.View
type PlatformDirection = ops.PlatformDirection
type PlatformDetail = ops.PlatformDetail
type StationDetail = ops.StationDetail
type ServicePlan = ops.ServicePlan
type ServiceTrip = ops.ServiceTrip
type ServiceView = ops.ServiceView
type TransferSite = ops.TransferSite
type Control = ops.Control
