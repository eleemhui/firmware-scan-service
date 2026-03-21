package service

import (
	"context"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// vulnerabilities collection uses a single document {_id: "global", cve_ids: [...]}
// $addToSet ensures uniqueness atomically across all replicas — no separate
// unique index or application-level deduplication needed.

const vulnsDocID = "global"

// AddVulns appends CVE IDs to the global registry, ignoring duplicates.
func AddVulns(ctx context.Context, database *mongo.Database, cveIDs []string) ([]string, error) {
	coll := database.Collection("vulnerabilities")
	_, err := coll.UpdateOne(
		ctx,
		bson.M{"_id": vulnsDocID},
		bson.M{"$addToSet": bson.M{"cve_ids": bson.M{"$each": cveIDs}}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return nil, fmt.Errorf("add vulns: %w", err)
	}
	return ListVulns(ctx, database)
}

// ListVulns returns all unique CVE IDs in sorted order.
func ListVulns(ctx context.Context, database *mongo.Database) ([]string, error) {
	var doc struct {
		CveIDs []string `bson:"cve_ids"`
	}
	err := database.Collection("vulnerabilities").
		FindOne(ctx, bson.M{"_id": vulnsDocID}).
		Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list vulns: %w", err)
	}
	sort.Strings(doc.CveIDs)
	return doc.CveIDs, nil
}
