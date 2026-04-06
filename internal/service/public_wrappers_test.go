package service

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// newDisconnectedDB returns a *mongo.Database that is not connected to any
// real server. Only use it in tests that fail on input validation before
// touching the DB — any test that executes a query will hang.
func newDisconnectedDB(t *testing.T) *mongo.Database {
	t.Helper()
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("mongo.Connect: %v", err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })
	return client.Database("test_firmware_db")
}

func TestClaimScan_Public_InvalidID(t *testing.T) {
	db := newDisconnectedDB(t)
	_, err := ClaimScan(context.Background(), db, "not-an-objectid")
	if err == nil {
		t.Error("expected error for invalid ObjectID")
	}
}

func TestCompleteScan_Public_InvalidID(t *testing.T) {
	db := newDisconnectedDB(t)
	err := CompleteScan(context.Background(), db, "not-an-objectid")
	if err == nil {
		t.Error("expected error for invalid ObjectID")
	}
}

func TestRecordVulnerabilities_Public_InvalidID(t *testing.T) {
	db := newDisconnectedDB(t)
	err := RecordVulnerabilities(context.Background(), db, "not-an-objectid", []string{"CVE-001"})
	if err == nil {
		t.Error("expected error for invalid ObjectID")
	}
}

func TestAddVulnsToRegistry_Public_EmptyList(t *testing.T) {
	db := newDisconnectedDB(t)
	// Empty list is a no-op — returns nil without touching the DB.
	err := AddVulnsToRegistry(context.Background(), db, []string{}, "scan-1")
	if err != nil {
		t.Errorf("unexpected error for empty CVE list: %v", err)
	}
}