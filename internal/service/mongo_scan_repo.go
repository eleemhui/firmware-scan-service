package service

import (
	"context"
	"time"

	"firmware-scan-service/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoScanRepo struct {
	coll *mongo.Collection
}

// NewMongoScanRepo returns a ScanRepository backed by MongoDB.
func NewMongoScanRepo(db *mongo.Database) ScanRepository {
	return &mongoScanRepo{coll: db.Collection("firmware_scans")}
}

func (r *mongoScanRepo) Insert(ctx context.Context, scan *model.FirmwareScan) (primitive.ObjectID, error) {
	result, err := r.coll.InsertOne(ctx, scan)
	if err != nil {
		return primitive.NilObjectID, err
	}
	return result.InsertedID.(primitive.ObjectID), nil
}

func (r *mongoScanRepo) FindByDeviceHash(ctx context.Context, deviceID, binaryHash string) (*model.FirmwareScan, error) {
	var scan model.FirmwareScan
	err := r.coll.FindOne(ctx, bson.M{"device_id": deviceID, "binary_hash": binaryHash}).Decode(&scan)
	if err != nil {
		return nil, err
	}
	return &scan, nil
}

func (r *mongoScanRepo) Claim(ctx context.Context, id primitive.ObjectID) (bool, error) {
	now := time.Now()
	result := r.coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id, "status": model.StatusScheduled},
		bson.M{"$set": bson.M{"status": model.StatusStarted, "scan_started_at": now, "updated_at": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	if result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	return result.Err() == nil, result.Err()
}

func (r *mongoScanRepo) Complete(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": model.StatusComplete, "scan_completed_at": now, "updated_at": now}},
	)
	return err
}

func (r *mongoScanRepo) SetVulns(ctx context.Context, id primitive.ObjectID, cveIDs []string) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"detected_vulns": cveIDs, "updated_at": time.Now()}},
	)
	return err
}

func (r *mongoScanRepo) FindStale(ctx context.Context, before time.Time) ([]model.FirmwareScan, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"status": model.StatusStarted, "updated_at": bson.M{"$lt": before}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var scans []model.FirmwareScan
	return scans, cursor.All(ctx, &scans)
}

func (r *mongoScanRepo) ResetToScheduled(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id, "status": model.StatusStarted},
		bson.M{"$set": bson.M{"status": model.StatusScheduled, "updated_at": time.Now()}},
	)
	return err
}

func (r *mongoScanRepo) FindOrphaned(ctx context.Context, createdBefore time.Time, requeueBefore time.Time, limit int64) ([]model.FirmwareScan, error) {
	cursor, err := r.coll.Find(ctx,
		bson.M{
			"status":     model.StatusScheduled,
			"created_at": bson.M{"$lt": createdBefore},
			"$or": bson.A{
				bson.M{"last_requeued_at": bson.M{"$exists": false}},
				bson.M{"last_requeued_at": bson.M{"$lt": requeueBefore}},
			},
		},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}).SetLimit(limit),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var scans []model.FirmwareScan
	return scans, cursor.All(ctx, &scans)
}

func (r *mongoScanRepo) StampRequeued(ctx context.Context, id primitive.ObjectID, at time.Time) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"last_requeued_at": at}},
	)
	return err
}