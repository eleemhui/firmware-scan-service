package service

import (
	"context"
	"time"

	"firmware-scan-service/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoVulnRepo struct {
	coll *mongo.Collection
}

// NewMongoVulnRepo returns a VulnRepository backed by MongoDB.
func NewMongoVulnRepo(db *mongo.Database) VulnRepository {
	return &mongoVulnRepo{coll: db.Collection("vulnerabilities")}
}

func (r *mongoVulnRepo) UpsertCVE(ctx context.Context, cveID, scanID string, now time.Time) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": cveID},
		bson.M{
			"$setOnInsert": bson.M{"first_detected": now, "first_detected_by": scanID},
			"$set":         bson.M{"last_detected": now, "last_detected_by": scanID},
			"$inc":         bson.M{"detected_count": 1},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *mongoVulnRepo) ListAll(ctx context.Context) ([]model.Vulnerability, error) {
	cursor, err := r.coll.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var vulns []model.Vulnerability
	return vulns, cursor.All(ctx, &vulns)
}