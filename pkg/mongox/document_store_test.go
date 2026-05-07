package mongox

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type documentStoreTestDoc struct {
	ID    string `bson:"_id"`
	Title string `bson:"title"`
}

func (documentStoreTestDoc) MongoCollectionName() string {
	return "document_store_notes"
}

func TestDocumentStoreExposesUnderlyingCollection(t *testing.T) {
	store := newDocumentStoreWithCollection(newCollectionWithOperator[documentStoreTestDoc]("custom_notes", &fakeCollectionOperator{}))

	require.Equal(t, "custom_notes", store.Collection().name)
}

func TestDocumentStoreUsesDocumentCollectionName(t *testing.T) {
	operator := &fakeCollectionOperator{}
	collection := newCollectionWithOperator[documentStoreTestDoc](documentStoreTestDoc{}.MongoCollectionName(), operator)
	store := newDocumentStoreWithCollection(collection)

	require.Equal(t, "document_store_notes", store.Collection().name)
}

func TestDocumentStoreForwardsCRUDToCollection(t *testing.T) {
	operator := &fakeCollectionOperator{
		findOneDocument: bson.D{{Key: "_id", Value: "note-1"}, {Key: "title", Value: "hello"}},
		findDocs: []any{
			bson.D{{Key: "_id", Value: "note-1"}, {Key: "title", Value: "hello"}},
			bson.D{{Key: "_id", Value: "note-2"}, {Key: "title", Value: "world"}},
		},
		countResult: 2,
	}
	store := newDocumentStoreWithCollection(newCollectionWithOperator[documentStoreTestDoc]("notes", operator))
	ctx := context.Background()
	doc := &documentStoreTestDoc{ID: "note-1", Title: "hello"}

	insertResult, err := store.Insert(ctx, doc)
	require.NoError(t, err)
	require.Equal(t, "created-id", insertResult.InsertedID)
	require.Equal(t, doc, operator.insertDocument)

	upsertResult, err := store.UpsertByID(ctx, "note-1", doc)
	require.NoError(t, err)
	require.Equal(t, int64(1), upsertResult.MatchedCount)
	require.Equal(t, bson.M{"_id": "note-1"}, operator.replaceFilter)
	require.Equal(t, doc, operator.replaceReplacement)

	found, err := store.FindByID(ctx, "note-1")
	require.NoError(t, err)
	require.Equal(t, "note-1", found.ID)
	require.Equal(t, bson.M{"_id": "note-1"}, operator.findOneFilter)

	many, err := store.FindMany(ctx, bson.M{"title": "hello"})
	require.NoError(t, err)
	require.Len(t, many, 2)
	require.Equal(t, bson.M{"title": "hello"}, operator.findFilter)

	updateResult, err := store.UpdateOne(ctx, bson.M{"_id": "note-1"}, bson.M{"$set": bson.M{"title": "updated"}})
	require.NoError(t, err)
	require.Equal(t, int64(1), updateResult.MatchedCount)
	require.Equal(t, bson.M{"_id": "note-1"}, operator.updateFilter)

	deleteResult, err := store.DeleteByID(ctx, "note-1")
	require.NoError(t, err)
	require.Equal(t, int64(1), deleteResult.DeletedCount)
	require.Equal(t, bson.M{"_id": "note-1"}, operator.deleteFilter)

	count, err := store.Count(ctx, bson.M{"title": "hello"})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	require.Equal(t, bson.M{"title": "hello"}, operator.countFilter)
}

func TestDocumentStoreForwardsMissingDocumentError(t *testing.T) {
	store := newDocumentStoreWithCollection(newCollectionWithOperator[documentStoreTestDoc]("notes", &fakeCollectionOperator{
		findOneDocument: bson.D{},
		findOneErr:      mongo.ErrNoDocuments,
	}))

	_, err := store.FindByID(context.Background(), "missing")

	require.ErrorIs(t, err, ErrNotFound)
}
