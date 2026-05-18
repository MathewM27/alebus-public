package busreadmodel

import "context"

type BusPositionDTO struct {
	Lat       float64
	Lon       float64
	Timestamp int64
	Accuracy  float64
	SpeedKmh  float64

	// OrderingTimestampMs is the per-bus monotonic ordering timestamp used for staleness guards.
	// For Redis live state this maps to Redis `updated_at` (ordering_ts_ms), not device time.
	OrderingTimestampMs int64

	// DeviceTimestampMs is the raw device-reported timestamp (untrusted metadata).
	DeviceTimestampMs int64

	// ReceivedAtMs is the server receive timestamp at the ingestion edge (trusted metadata).
	ReceivedAtMs int64
}

type BusDTO struct {
	BusID               string
	OperatorID          string
	RouteID             string
	Direction           int
	StopIndex           int
	Status              int
	IsAtTerminal        bool
	TerminalArrivalTime string
	Position            BusPositionDTO
	UpdatedAt           string
}

type GetBusRequest struct{ BusID string }

type ListBusesRequest struct {
	RouteID    string
	OperatorID string
	Status     *int
}

type BusReader interface {
	GetBus(ctx context.Context, req GetBusRequest) (BusDTO, bool, error)
	ListBuses(ctx context.Context, req ListBusesRequest) ([]BusDTO, error)
}
