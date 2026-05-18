package usecase_test

import (
	"context"
	"testing"
	"time"

	busreadmodel "github.com/MathewM27/busTrack-alebus/application/bus/readmodel"
	"github.com/MathewM27/busTrack-alebus/application/bus/streaming"
	"github.com/MathewM27/busTrack-alebus/application/bus/usecase"
)

type fakeUpdatesSource struct {
	ch chan streaming.BusUpdate
}

func (f *fakeUpdatesSource) Subscribe(ctx context.Context) <-chan streaming.BusUpdate {
	return f.ch
}

func TestStreamLiveBusUpdates_FiltersByRouteID(t *testing.T) {
	src := &fakeUpdatesSource{ch: make(chan streaming.BusUpdate, 10)}
	u := usecase.NewStreamLiveBusUpdatesUseCase(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filter := usecase.NewLiveBusStreamFilter("route-a", nil)
	out := u.Stream(ctx, filter)

	src.ch <- streaming.BusUpdate{Bus: busreadmodel.BusDTO{BusID: "b1", RouteID: "route-a"}, ServerTs: time.Now()}
	src.ch <- streaming.BusUpdate{Bus: busreadmodel.BusDTO{BusID: "b2", RouteID: "route-b"}, ServerTs: time.Now()}

	select {
	case got := <-out:
		if got.Bus.BusID != "b1" {
			t.Fatalf("expected b1, got %q", got.Bus.BusID)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for routed update")
	}

	select {
	case got := <-out:
		t.Fatalf("expected no second update, got %q", got.Bus.BusID)
	case <-time.After(100 * time.Millisecond):
		// ok
	}
}

func TestStreamLiveBusUpdates_FiltersByBusIDs(t *testing.T) {
	src := &fakeUpdatesSource{ch: make(chan streaming.BusUpdate, 10)}
	u := usecase.NewStreamLiveBusUpdatesUseCase(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filter := usecase.NewLiveBusStreamFilter("", []string{"b2", "b3"})
	out := u.Stream(ctx, filter)

	src.ch <- streaming.BusUpdate{Bus: busreadmodel.BusDTO{BusID: "b1", RouteID: "r"}, ServerTs: time.Now()}
	src.ch <- streaming.BusUpdate{Bus: busreadmodel.BusDTO{BusID: "b2", RouteID: "r"}, ServerTs: time.Now()}

	select {
	case got := <-out:
		if got.Bus.BusID != "b2" {
			t.Fatalf("expected b2, got %q", got.Bus.BusID)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for busId-filtered update")
	}
}

func TestStreamLiveBusUpdates_ClosesOnContextCancel(t *testing.T) {
	src := &fakeUpdatesSource{ch: make(chan streaming.BusUpdate)}
	u := usecase.NewStreamLiveBusUpdatesUseCase(src)

	ctx, cancel := context.WithCancel(context.Background())
	out := u.Stream(ctx, usecase.LiveBusStreamFilter{})

	cancel()

	select {
	case _, ok := <-out:
		if ok {
			t.Fatalf("expected stream to close")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for stream close")
	}
}
