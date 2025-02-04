package usecases

import (
	"github.com/lintang-b-s/osm-search/pkg/datastructure"
)

type Searcher interface {
	FreeFormQuery(query string, k, offset int) ([]datastructure.Node, error)
	Autocomplete(query string, k, offset int) ([]datastructure.Node, error)
	ReverseGeocoding(lat, lon float64) (datastructure.Node, error)
}
