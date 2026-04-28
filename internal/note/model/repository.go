package model

import "context"

// Repository defines persistence behavior required by the note service layer.
type Repository interface {
	Save(ctx context.Context, note *Note) error
	FindByID(ctx context.Context, id string) (*Note, error)
}
