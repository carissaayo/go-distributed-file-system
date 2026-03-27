package store

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{}
}
