package service

import (
	"context"
	"testing"
	"time"

	"web3/internal/model"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type clientEventRepositoryStub struct {
	event *model.ClientEvent
}

func (s *clientEventRepositoryStub) CreateClientEvent(_ context.Context, event *model.ClientEvent) error {
	s.event = event
	return nil
}

type clientEventLimiterStub struct {
	allowed bool
	key     string
}

func (s *clientEventLimiterStub) Allow(_ context.Context, key string) (bool, error) {
	s.key = key
	return s.allowed, nil
}

func TestClientEventServiceStoresOnlyInstallationHash(t *testing.T) {
	repository := new(clientEventRepositoryStub)
	limiter := &clientEventLimiterStub{allowed: true}
	service, err := NewClientEventService(repository, limiter)
	if err != nil {
		t.Fatalf("NewClientEventService() error = %v", err)
	}
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	installationID := "c85d5517-92b8-4cb7-93eb-266b1e738631"

	_, err = service.RecordClientEvent(context.Background(), &web3v1.RecordClientEventRequest{
		InstallationId: installationID,
		EventType:      web3v1.ClientEventType_CLIENT_EVENT_TYPE_QUERY_SUCCEEDED,
		AppVersion:     "1.0.0",
		Platform:       web3v1.ClientPlatform_CLIENT_PLATFORM_WEB,
		Network:        "tron-mainnet",
		ResultType:     "success",
		DurationBucket: web3v1.DurationBucket_DURATION_BUCKET_ONE_TO_FIVE_SECONDS,
		OccurredAt:     timestamppb.New(now),
	})
	if err != nil {
		t.Fatalf("RecordClientEvent() error = %v", err)
	}
	if repository.event == nil {
		t.Fatal("event was not stored")
	}
	if repository.event.InstallationHash == installationID || len(repository.event.InstallationHash) != 64 {
		t.Fatalf("installation hash = %q", repository.event.InstallationHash)
	}
	if limiter.key != repository.event.InstallationHash {
		t.Fatalf("limiter key = %q, stored hash = %q", limiter.key, repository.event.InstallationHash)
	}
}

func TestClientEventServiceRejectsRateLimit(t *testing.T) {
	service, _ := NewClientEventService(new(clientEventRepositoryStub), &clientEventLimiterStub{allowed: false})
	now := time.Now().UTC()
	_, err := service.RecordClientEvent(context.Background(), &web3v1.RecordClientEventRequest{
		InstallationId: "c85d5517-92b8-4cb7-93eb-266b1e738631",
		EventType:      web3v1.ClientEventType_CLIENT_EVENT_TYPE_FIRST_OPEN,
		Platform:       web3v1.ClientPlatform_CLIENT_PLATFORM_IOS,
		OccurredAt:     timestamppb.New(now),
	})
	if err == nil {
		t.Fatal("RecordClientEvent() error = nil, want rate-limit error")
	}
}
