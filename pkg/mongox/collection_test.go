package mongox

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type fakeCollectionOperator struct {
	insertDocument any
	insertResult   *mongo.InsertOneResult
	insertErr      error

	replaceFilter      any
	replaceReplacement any
	replaceOptions     []options.Lister[options.ReplaceOptions]
	replaceResult      *mongo.UpdateResult
	replaceErr         error

	findOneFilter   any
	findOneDocument any
	findOneErr      error

	findFilter  any
	findOptions []options.Lister[options.FindOptions]
	findDocs    []any
	findErr     error

	updateFilter  any
	updatePayload any
	updateResult  *mongo.UpdateResult
	updateErr     error

	deleteFilter any
	deleteResult *mongo.DeleteResult
	deleteErr    error

	countFilter any
	countResult int64
	countErr    error
}

func (f *fakeCollectionOperator) InsertOne(ctx context.Context, document any, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	f.insertDocument = document
	if f.insertResult != nil || f.insertErr != nil {
		return f.insertResult, f.insertErr
	}
	return &mongo.InsertOneResult{InsertedID: "created-id"}, nil
}

func (f *fakeCollectionOperator) ReplaceOne(ctx context.Context, filter any, replacement any, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	f.replaceFilter = filter
	f.replaceReplacement = replacement
	f.replaceOptions = opts
	if f.replaceResult != nil || f.replaceErr != nil {
		return f.replaceResult, f.replaceErr
	}
	return &mongo.UpdateResult{MatchedCount: 1, ModifiedCount: 1}, nil
}

func (f *fakeCollectionOperator) FindOne(ctx context.Context, filter any, opts ...options.Lister[options.FindOneOptions]) *mongo.SingleResult {
	f.findOneFilter = filter
	return mongo.NewSingleResultFromDocument(f.findOneDocument, f.findOneErr, nil)
}

func (f *fakeCollectionOperator) Find(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) (*mongo.Cursor, error) {
	f.findFilter = filter
	f.findOptions = opts
	if f.findErr != nil {
		return nil, f.findErr
	}
	return mongo.NewCursorFromDocuments(f.findDocs, nil, nil)
}

func (f *fakeCollectionOperator) UpdateOne(ctx context.Context, filter any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	f.updateFilter = filter
	f.updatePayload = update
	if f.updateResult != nil || f.updateErr != nil {
		return f.updateResult, f.updateErr
	}
	return &mongo.UpdateResult{MatchedCount: 1, ModifiedCount: 1}, nil
}

func (f *fakeCollectionOperator) DeleteOne(ctx context.Context, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	f.deleteFilter = filter
	if f.deleteResult != nil || f.deleteErr != nil {
		return f.deleteResult, f.deleteErr
	}
	return &mongo.DeleteResult{DeletedCount: 1}, nil
}

func (f *fakeCollectionOperator) CountDocuments(ctx context.Context, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	f.countFilter = filter
	return f.countResult, f.countErr
}

type collectionTestDoc struct {
	ID    string `bson:"_id"`
	Title string `bson:"title"`
}

func TestCollectionFindByIDDecodesDocument(t *testing.T) {
	operator := &fakeCollectionOperator{
		findOneDocument: bson.D{{Key: "_id", Value: "note-1"}, {Key: "title", Value: "hello"}},
	}
	collection := newCollectionWithOperator[collectionTestDoc]("notes", operator)

	doc, err := collection.FindByID(context.Background(), "note-1")

	require.NoError(t, err)
	require.Equal(t, "note-1", doc.ID)
	require.Equal(t, "hello", doc.Title)
	require.Equal(t, bson.M{"_id": "note-1"}, operator.findOneFilter)
}

func TestCollectionFindByIDMapsMissingDocument(t *testing.T) {
	collection := newCollectionWithOperator[collectionTestDoc]("notes", &fakeCollectionOperator{
		findOneDocument: bson.D{},
		findOneErr:      mongo.ErrNoDocuments,
	})

	_, err := collection.FindByID(context.Background(), "missing")

	require.ErrorIs(t, err, ErrNotFound)
}

func TestCollectionUpsertByIDUsesReplaceWithUpsert(t *testing.T) {
	operator := &fakeCollectionOperator{}
	collection := newCollectionWithOperator[collectionTestDoc]("notes", operator)
	doc := &collectionTestDoc{ID: "note-1", Title: "hello"}

	result, err := collection.UpsertByID(context.Background(), "note-1", doc)

	require.NoError(t, err)
	require.Equal(t, int64(1), result.MatchedCount)
	require.Equal(t, bson.M{"_id": "note-1"}, operator.replaceFilter)
	require.Equal(t, doc, operator.replaceReplacement)

	var args options.ReplaceOptions
	require.Len(t, operator.replaceOptions, 1)
	for _, apply := range operator.replaceOptions[0].List() {
		require.NoError(t, apply(&args))
	}
	require.NotNil(t, args.Upsert)
	require.True(t, *args.Upsert)
}

func TestCollectionFindManyDecodesDocuments(t *testing.T) {
	operator := &fakeCollectionOperator{
		findDocs: []any{
			bson.D{{Key: "_id", Value: "note-1"}, {Key: "title", Value: "one"}},
			bson.D{{Key: "_id", Value: "note-2"}, {Key: "title", Value: "two"}},
		},
	}
	collection := newCollectionWithOperator[collectionTestDoc]("notes", operator)

	docs, err := collection.FindMany(context.Background(), bson.M{"author_id": "user-1"}, options.Find().SetLimit(20))

	require.NoError(t, err)
	require.Len(t, docs, 2)
	require.Equal(t, "note-1", docs[0].ID)
	require.Equal(t, "note-2", docs[1].ID)
	require.Equal(t, bson.M{"author_id": "user-1"}, operator.findFilter)
	require.Len(t, operator.findOptions, 1)
}

func TestCollectionForwardsWriteErrors(t *testing.T) {
	writeErr := errors.New("write failed")
	collection := newCollectionWithOperator[collectionTestDoc]("notes", &fakeCollectionOperator{replaceErr: writeErr})

	_, err := collection.UpsertByID(context.Background(), "note-1", &collectionTestDoc{ID: "note-1"})

	require.ErrorIs(t, err, writeErr)
}
