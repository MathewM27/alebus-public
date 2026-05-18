package usecase

import (
	"context"
	"strings"

	busports "github.com/MathewM27/busTrack-alebus/application/bus/ports"
	"github.com/MathewM27/busTrack-alebus/domain/repositories"
	"github.com/MathewM27/busTrack-alebus/domain/types"
)

type BusStreamAuthorizationPolicy struct {
	busRepo   repositories.BusRepository
	routeRepo repositories.RouteRepository
}

var _ busports.StreamAuthorizationPolicy = (*BusStreamAuthorizationPolicy)(nil)

func NewBusStreamAuthorizationPolicy(busRepo repositories.BusRepository, routeRepo repositories.RouteRepository) *BusStreamAuthorizationPolicy {
	return &BusStreamAuthorizationPolicy{busRepo: busRepo, routeRepo: routeRepo}
}

func (p *BusStreamAuthorizationPolicy) CanSubscribeToRoute(ctx context.Context, operatorID string, routeID string) (bool, error) {
	if p == nil || p.routeRepo == nil {
		return false, nil
	}
	operatorID = strings.TrimSpace(operatorID)
	routeID = strings.TrimSpace(routeID)
	if operatorID == "" || routeID == "" {
		return false, nil
	}
	route, err := p.routeRepo.FindByID(ctx, types.RouteID(routeID))
	if err != nil {
		return false, err
	}
	if route == nil {
		return false, nil
	}
	for _, oid := range route.OperatorIDs() {
		if string(oid) == operatorID {
			return true, nil
		}
	}
	return false, nil
}

func (p *BusStreamAuthorizationPolicy) CanSubscribeToBus(ctx context.Context, operatorID string, busID string) (bool, error) {
	if p == nil || p.busRepo == nil {
		return false, nil
	}
	operatorID = strings.TrimSpace(operatorID)
	busID = strings.TrimSpace(busID)
	if operatorID == "" || busID == "" {
		return false, nil
	}
	bus, err := p.busRepo.FindByID(ctx, types.BusID(busID))
	if err != nil {
		return false, err
	}
	if bus == nil {
		return false, nil
	}
	return string(bus.OperatorID()) == operatorID, nil
}
