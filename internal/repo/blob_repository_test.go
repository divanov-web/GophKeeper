package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlobRepository_CreateIfAbsent_Idempotent(t *testing.T) {
	db := newTestDB(t)
	r := NewBlobRepository(db)
	ctx := context.Background()

	// первая вставка — created=true
	created, err := r.CreateIfAbsent(ctx, "b1", []byte{1, 2}, []byte{3})
	assert.NoError(t, err)
	assert.True(t, created)

	// повторная — created=false
	created, err = r.CreateIfAbsent(ctx, "b1", []byte{9}, []byte{9})
	assert.NoError(t, err)
	assert.False(t, created)
}
