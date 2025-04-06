package repository

import (
	"context"
	"time"

	"github.com/logankrause16/email_processing/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDomainRepository is a MongoDB implementation of DomainRepository
type MongoDomainRepository struct {
	client     *mongo.Client
	db         *mongo.Database
	collection *mongo.Collection
}

// NewMongoDomainRepository creates a new MongoDB domain repository
func NewMongoDomainRepository(ctx context.Context, connectionString string, dbName string) (*MongoDomainRepository, error) {
	// Set client options
	clientOptions := options.Client().ApplyURI(connectionString)

	// Create a timeout context for the connection
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Connect to MongoDB
	client, err := mongo.Connect(connCtx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Check the connection
	err = client.Ping(connCtx, nil)
	if err != nil {
		return nil, err
	}

	// Create indexes for better performance
	db := client.Database(dbName)
	collection := db.Collection("domains")

	// Create index on domain name for faster lookups (this is already the primary key _id)
	// Also create an index on status field for queries by status
	_, err = collection.Indexes().CreateMany(
		ctx,
		[]mongo.IndexModel{
			{
				Keys:    bson.D{{"status", 1}},
				Options: options.Index().SetName("status_regular_index"),
			},
			{
				Keys:    bson.D{{"updated_at", -1}},
				Options: options.Index().SetName("updated_at_index"),
			},
		},
	)
	if err != nil {
		return nil, err
	}

	return &MongoDomainRepository{
		client:     client,
		db:         db,
		collection: collection,
	}, nil
}

// GetDomainInfo retrieves information about a domain
func (r *MongoDomainRepository) GetDomainInfo(ctx context.Context, domainName string) (*domain.DomainInfo, error) {
	var domainInfo domain.DomainInfo

	// Create a timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Find domain by name (_id)
	err := r.collection.FindOne(opCtx, bson.M{"_id": domainName}).Decode(&domainInfo)
	if err != nil {
		// If domain not found, return default domain info
		if err == mongo.ErrNoDocuments {
			return &domain.DomainInfo{
				Name:           domainName,
				DeliveredCount: 0,
				BouncedCount:   0,
				Status:         domain.StatusUnknown,
				UpdatedAt:      time.Now(),
			}, nil
		}
		return nil, err
	}

	return &domainInfo, nil
}

// IncrementEventCount increments the count for a specific event type
func (r *MongoDomainRepository) IncrementEventCount(ctx context.Context, domainName string, eventType domain.EventType) error {
	// Create a timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Define the update
	update := bson.M{
		"$inc": bson.M{},
		"$set": bson.M{"updated_at": time.Now()},
	}

	// Set the appropriate counter to increment
	switch eventType {
	case domain.EventDelivered:
		update["$inc"].(bson.M)["delivered_count"] = 1
	case domain.EventBounced:
		update["$inc"].(bson.M)["bounced_count"] = 1
	}

	// Use upsert to create the document if it doesn't exist
	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(
		opCtx,
		bson.M{"_id": domainName},
		update,
		opts,
	)

	return err
}

// UpdateDomainStatus updates the status of a domain
func (r *MongoDomainRepository) UpdateDomainStatus(ctx context.Context, domainName string, status domain.DomainStatus) error {
	// Create a timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(
		opCtx,
		bson.M{"_id": domainName},
		update,
		opts,
	)

	return err
}

// Close closes the MongoDB connection
func (r *MongoDomainRepository) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}

// GetDomainStats gets stats about domains in the database
func (r *MongoDomainRepository) GetDomainStats(ctx context.Context) (map[string]int64, error) {
	// Create a timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Pipeline for aggregation
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":   "$status",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := r.collection.Aggregate(opCtx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Process results
	stats := make(map[string]int64)
	for cursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		stats[result.ID] = result.Count
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	// Set default values for statuses that don't exist
	if _, ok := stats[string(domain.StatusCatchAll)]; !ok {
		stats[string(domain.StatusCatchAll)] = 0
	}
	if _, ok := stats[string(domain.StatusNotCatchAll)]; !ok {
		stats[string(domain.StatusNotCatchAll)] = 0
	}
	if _, ok := stats[string(domain.StatusUnknown)]; !ok {
		stats[string(domain.StatusUnknown)] = 0
	}

	// Count total domains
	totalCount, err := r.collection.CountDocuments(opCtx, bson.M{})
	if err != nil {
		return nil, err
	}
	stats["total"] = totalCount

	return stats, nil
}

// IncrementEventCountBatch increments the counts for multiple domains in a batch
func (r *MongoDomainRepository) IncrementEventCountBatch(ctx context.Context, events []EventBatch) error {
	if len(events) == 0 {
		return nil
	}

	// Create a timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Create bulk write models
	bulkOps := make([]mongo.WriteModel, 0, len(events))
	now := time.Now()

	for _, event := range events {
		// Define which field to increment
		updateField := "delivered_count"
		if event.EventType == domain.EventBounced {
			updateField = "bounced_count"
		}

		// Create update model
		model := mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": event.DomainName}).
			SetUpdate(bson.M{
				"$inc": bson.M{updateField: event.Count},
				"$set": bson.M{"updated_at": now},
			}).
			SetUpsert(true)

		bulkOps = append(bulkOps, model)
	}

	// Execute bulk write with unordered option for better performance
	opts := options.BulkWrite().SetOrdered(false)
	_, err := r.collection.BulkWrite(opCtx, bulkOps, opts)
	return err
}

// UpdateStatusBatch updates the status for multiple domains in a batch
func (r *MongoDomainRepository) UpdateStatusBatch(ctx context.Context, updates map[string]domain.DomainStatus) error {
	if len(updates) == 0 {
		return nil
	}

	// Create a timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Create bulk write models
	bulkOps := make([]mongo.WriteModel, 0, len(updates))
	now := time.Now()

	for domainName, status := range updates {
		model := mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": domainName}).
			SetUpdate(bson.M{
				"$set": bson.M{
					"status":     status,
					"updated_at": now,
				},
			}).
			SetUpsert(true)

		bulkOps = append(bulkOps, model)
	}

	// Execute bulk write with unordered option for better performance
	opts := options.BulkWrite().SetOrdered(false)
	_, err := r.collection.BulkWrite(opCtx, bulkOps, opts)
	return err
}

// GetDomainsStatusBatch retrieves information about multiple domains in a batch
func (r *MongoDomainRepository) GetDomainsStatusBatch(ctx context.Context, domainNames []string) (map[string]*domain.DomainInfo, error) {
	if len(domainNames) == 0 {
		return map[string]*domain.DomainInfo{}, nil
	}

	// Create a timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Create filter for multiple domain names
	filter := bson.M{"_id": bson.M{"$in": domainNames}}

	// Execute find query
	cursor, err := r.collection.Find(opCtx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(opCtx)

	// Process results
	result := make(map[string]*domain.DomainInfo)
	for cursor.Next(opCtx) {
		var domainInfo domain.DomainInfo
		if err := cursor.Decode(&domainInfo); err != nil {
			return nil, err
		}
		result[domainInfo.Name] = &domainInfo
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	// Fill in missing domains with default values
	now := time.Now()
	for _, name := range domainNames {
		if _, exists := result[name]; !exists {
			result[name] = &domain.DomainInfo{
				Name:           name,
				DeliveredCount: 0,
				BouncedCount:   0,
				Status:         domain.StatusUnknown,
				UpdatedAt:      now,
			}
		}
	}

	return result, nil
}
