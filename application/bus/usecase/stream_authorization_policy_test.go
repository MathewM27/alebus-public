package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MathewM27/busTrack-alebus/domain/aggregates"
	"github.com/MathewM27/busTrack-alebus/domain/enums"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
	"github.com/MathewM27/busTrack-alebus/domain/types"
	"github.com/MathewM27/busTrack-alebus/domain/valueobjects"
)

type nopEventRecorder struct{}

func (nopEventRecorder) Record(types.DomainEvent) error { return nil }

type fakeRouteRepo struct {
	routes map[types.RouteID]*aggregates.Route
	err    error
}

var _ repositories.RouteRepository = (*fakeRouteRepo)(nil)

func (f *fakeRouteRepo) Save(ctx context.Context, route *aggregates.Route) error { return nil }
func (f *fakeRouteRepo) FindActiveRoutes(ctx context.Context, at time.Time) ([]*aggregates.Route, error) {
	return nil, nil
}

func (f *fakeRouteRepo) FindByID(ctx context.Context, id types.RouteID) (*aggregates.Route, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.routes == nil {
		return nil, nil
	}
	return f.routes[id], nil
}

type fakeBusRepo struct {
	buses map[types.BusID]*aggregates.Bus
	err   error
}

var _ repositories.BusRepository = (*fakeBusRepo)(nil)

func (f *fakeBusRepo) Save(ctx context.Context, bus *aggregates.Bus) error { return nil }
func (f *fakeBusRepo) FindByID(ctx context.Context, id types.BusID) (*aggregates.Bus, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.buses == nil {
		return nil, nil
	}
	return f.buses[id], nil
}

func mustRoute(t *testing.T, id string, operatorIDs ...string) *aggregates.Route {
	t.Helper()
	ops := make([]types.OperatorID, 0, len(operatorIDs))
	for _, oid := range operatorIDs {
		ops = append(ops, types.OperatorID(oid))
	}

	r, err := aggregates.NewRoute(
		types.RouteID(id),
		ops,
		[]valueobjects.Stop{
			{ID: "s1", Name: "s1", Location: types.GeoLocation{Latitude: 1, Longitude: 1}},
			{ID: "s2", Name: "s2", Location: types.GeoLocation{Latitude: 2, Longitude: 2}},
		},
		"route",
		enums.RouteDirectionBidirectional,
		enums.RouteTypeUrban,
		time.Now().Add(-time.Hour),
		time.Now().Add(time.Hour),
		time.Now().Add(-time.Minute),
		nopEventRecorder{},
	)
	if err != nil {
		t.Fatalf("NewRoute: %v", err)
	}
	return r
}

func mustBus(t *testing.T, id, operatorID, routeID string) *aggregates.Bus {
	t.Helper()
	b, err := aggregates.NewBus(
		types.BusID(id),
		types.OperatorID(operatorID),
		types.RouteID(routeID),
		types.PositionSnapshot{Location: types.GeoLocation{Latitude: 1, Longitude: 1}, Timestamp: time.Now().UTC()},
		0,
		enums.DirectionOutbound,
		time.Now().Add(-time.Minute),
		nopEventRecorder{},
	)
	if err != nil {
		t.Fatalf("NewBus: %v", err)
	}
	return b
}

func TestBusStreamAuthorizationPolicy_AllowsRouteOperator(t *testing.T) {
	routeRepo := &fakeRouteRepo{routes: map[types.RouteID]*aggregates.Route{
		types.RouteID("r1"): mustRoute(t, "r1", "op1"),
	}}
	busRepo := &fakeBusRepo{}
	p := NewBusStreamAuthorizationPolicy(busRepo, routeRepo)

	allowed, err := p.CanSubscribeToRoute(context.Background(), "op1", "r1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed")
	}
}

func TestBusStreamAuthorizationPolicy_DeniesWrongOperatorRoute(t *testing.T) {
	routeRepo := &fakeRouteRepo{routes: map[types.RouteID]*aggregates.Route{
		types.RouteID("r1"): mustRoute(t, "r1", "op1"),
	}}
	busRepo := &fakeBusRepo{}
	p := NewBusStreamAuthorizationPolicy(busRepo, routeRepo)

	allowed, err := p.CanSubscribeToRoute(context.Background(), "op2", "r1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if allowed {
		t.Fatalf("expected denied")
	}
}

func TestBusStreamAuthorizationPolicy_AllowsBusOperator(t *testing.T) {
	busRepo := &fakeBusRepo{buses: map[types.BusID]*aggregates.Bus{
		types.BusID("b1"): mustBus(t, "b1", "op1", "r1"),
	}}
	routeRepo := &fakeRouteRepo{}
	p := NewBusStreamAuthorizationPolicy(busRepo, routeRepo)

	allowed, err := p.CanSubscribeToBus(context.Background(), "op1", "b1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed")
	}
}

func TestBusStreamAuthorizationPolicy_DeniesWrongOperatorBus(t *testing.T) {
	busRepo := &fakeBusRepo{buses: map[types.BusID]*aggregates.Bus{
		types.BusID("b1"): mustBus(t, "b1", "op1", "r1"),
	}}
	routeRepo := &fakeRouteRepo{}
	p := NewBusStreamAuthorizationPolicy(busRepo, routeRepo)

	allowed, err := p.CanSubscribeToBus(context.Background(), "op2", "b1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if allowed {
		t.Fatalf("expected denied")
	}
}

func TestBusStreamAuthorizationPolicy_PropagatesRepoError(t *testing.T) {
	errDB := errors.New("db error")
	busRepo := &fakeBusRepo{err: errDB}
	routeRepo := &fakeRouteRepo{}
	p := NewBusStreamAuthorizationPolicy(busRepo, routeRepo)

	allowed, err := p.CanSubscribeToBus(context.Background(), "op1", "b1")
	if !errors.Is(err, errDB) {
		t.Fatalf("expected database error, got %v", err)
	}
	if allowed {
		t.Fatalf("expected denied")
	}
}
