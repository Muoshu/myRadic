package reverse_index

import "myRadic/types"

type IReverseIndexer interface {
	Add(doc types.Document)
	Delete(IntId uint64, keywords *types.Keyword)
	Search(q *types.TermQuery, onFlag uint64, offFlag uint64, orFlags []uint64) []string
}
