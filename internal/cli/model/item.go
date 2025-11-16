package model

// Item — base item model.
type Item struct {
	ID        string
	Name      string
	CreatedAt int64
	UpdatedAt int64
	Version   int64
	Deleted   bool
}
