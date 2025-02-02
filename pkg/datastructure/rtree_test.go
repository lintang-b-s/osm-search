package datastructure

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/rand"
)

// this is trash

func traverseRtreeAndTestIfBoundingBoxCorrect(node *RtreeNode, countLeaf *int, t *testing.T) {
	if node.IsLeaf {
		maxBB := node.Items[0].getBound()
		for _, item := range node.Items {
			*countLeaf++
			bb := item.getBound()
			maxBB = stretch(maxBB, bb)
		}

		if !node.Bound.isBBSame(maxBB) {
			t.Errorf("Bounding box not same")
		}
	} else {
		maxBB := node.Items[0].getBound()

		for _, item := range node.Items {
			bb := item.getBound()
			maxBB = stretch(maxBB, bb)
			traverseRtreeAndTestIfBoundingBoxCorrect(item, countLeaf, t)
		}

		if !node.Bound.isBBSame(maxBB) {
			t.Errorf("Bounding box not same")
		}
	}
}

func TestInsertRtree(t *testing.T) {
	itemsData := []OSMObject{}
	for i := 0; i < 100; i++ {
		itemsData = append(itemsData, OSMObject{
			ID: i,
			Lat:  float64(i),
			Lon:  float64(i),
		})
	}

	tests := []struct {
		name        string
		items       []OSMObject
		expectItems int
	}{
		{
			name: "Insert 100 item",
			items: append(itemsData, []OSMObject{
				{
					ID: 100,
					Lat:  0,
					Lon:  -5,
				},
				{
					ID: 101,
					Lat:  2,
					Lon:  -10,
				},
				{
					ID: 102,
					Lat:  3,
					Lon:  -15,
				},
			}...),
			expectItems: 103,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := NewRtree(25, 50, 2)
			for _, item := range tt.items {
				rt.InsertLeaf(item.GetBound(), item)

			}
			assert.Equal(t, 103, rt.Size)

			countLeaf := 0
			traverseRtreeAndTestIfBoundingBoxCorrect(rt.Root, &countLeaf, t)
			assert.Equal(t, tt.expectItems, countLeaf)
		})
	}

	t.Run("Insert 5 items", func(t *testing.T) {
		rt := NewRtree(25, 50, 2)
		for i := 0; i < 5; i++ {
			item := itemsData[i]

			rt.InsertLeaf(item.GetBound(), item)
		}
		assert.Equal(t, 5, rt.Size)
		root := rt.Root
		for i, item := range root.Items {
			assert.Equal(t, itemsData[i].ID, item.Leaf.ID)
		}

		countLeaf := 0
		traverseRtreeAndTestIfBoundingBoxCorrect(rt.Root, &countLeaf, t)
		assert.Equal(t, 5, countLeaf)
	})

}

func TestChooseSubtree(t *testing.T) {
	t.Run("Choose subtree", func(t *testing.T) {
		items := []*RtreeNode{
			&RtreeNode{
				Bound: NewRtreeBoundingBox(2, []float64{-1, -1}, []float64{1, 1}),
				Items: []*RtreeNode{
					&RtreeNode{
						Bound:  NewRtreeBoundingBox(2, []float64{-1, -1}, []float64{1, 1}),
						Items:  []*RtreeNode{},
						IsLeaf: true,
					},
					&RtreeNode{
						Bound:  NewRtreeBoundingBox(2, []float64{0, 0}, []float64{0, 0}),
						Items:  []*RtreeNode{},
						IsLeaf: true,
					},
				},
				IsLeaf: false,
			},

			&RtreeNode{
				Bound: NewRtreeBoundingBox(2, []float64{10, 10}, []float64{20, 20}),
				Items: []*RtreeNode{
					&RtreeNode{
						Bound:  NewRtreeBoundingBox(2, []float64{10, 10}, []float64{20, 20}),
						Items:  []*RtreeNode{},
						IsLeaf: true,
					},
				},
				IsLeaf: false,
			},
		}

		rt := NewRtree(1, 2, 2)
		rt.Root = &RtreeNode{
			Bound: NewRtreeBoundingBox(2, []float64{0, 0}, []float64{0, 0}),
			Items: items,
		}

		for _, item := range items {
			rt.Root.Bound = stretch(rt.Root.Bound, item.getBound())
		}

		newBB := NewRtreeBoundingBox(2, []float64{12, 12}, []float64{18, 18})

		rt.Root.Bound = stretch(rt.Root.Bound, newBB)

		choosedNode := rt.chooseSubtree(rt.Root, newBB)
		assert.Equal(t, items[1].Items[0], choosedNode)
	})

}

func TestSearch(t *testing.T) {

	t.Run("Search", func(t *testing.T) {
		itemsData := []OSMObject{}
		for i := 0; i < 100; i++ {
			itemsData = append(itemsData, OSMObject{
				ID: i,
				Lat:  float64(i),
				Lon:  float64(i),
			})
		}

		rt := NewRtree(10, 25, 2)
		for _, item := range itemsData {

			rt.InsertLeaf(item.GetBound(), item)

		}

		countLeaf := 0
		traverseRtreeAndTestIfBoundingBoxCorrect(rt.Root, &countLeaf, t)

		results := rt.Search(NewRtreeBoundingBox(2, []float64{0, 0}, []float64{50, 50}))

		for _, item := range results {

			itembb := item.getBound()
			if !overlaps(itembb, NewRtreeBoundingBox(2, []float64{0, 0}, []float64{50, 50})) {
				t.Errorf("Bounding box is not correct")

			}
		}
	})
}

func TestSplit(t *testing.T) {
	t.Run("Split", func(t *testing.T) {
		itemsData := []OSMObject{}
		for i := 0; i < 26; i++ {
			itemsData = append(itemsData, OSMObject{
				ID: i,
				Lat:  float64(i),
				Lon:  float64(i),
			})
		}

		rt := NewRtree(10, 25, 2)

		rt.InsertLeaf(itemsData[0].GetBound(), itemsData[0])
		for i := 1; i < 26; i++ {
			item := itemsData[i]

			newLeaf := &RtreeNode{Leaf: item, Bound: item.GetBound()}
			rt.Root.Items = append(rt.Root.Items, newLeaf)
		}

		newNode := rt.split(rt.Root)

		assert.Less(t, len(newNode.Items), 25)
		assert.Less(t, len(rt.Root.Items), 25)

		countLeaf := 0
		traverseRtreeAndTestIfBoundingBoxCorrect(rt.Root, &countLeaf, t)
		traverseRtreeAndTestIfBoundingBoxCorrect(newNode, &countLeaf, t)
	})
}

func TestOverflowTreatment(t *testing.T) {
	t.Run("Overflow treatment", func(t *testing.T) {
		itemsData := []OSMObject{}
		for i := 0; i < 26; i++ {
			itemsData = append(itemsData, OSMObject{
				ID: i,
				Lat:  float64(i),
				Lon:  float64(i),
			})
		}

		rt := NewRtree(10, 25, 2)

		rt.InsertLeaf(itemsData[0].GetBound(), itemsData[0])
		for i := 1; i < 26; i++ {
			item := itemsData[i]
			newLeaf := &RtreeNode{Leaf: item, Bound: item.GetBound()}
			rt.Root.Items = append(rt.Root.Items, newLeaf)
		}

		oldRoot := rt.Root
		rt.overflowTreatment(rt.Root, true)

		assert.NotEqual(t, oldRoot, rt.Root)
		assert.Equal(t, 2, len(rt.Root.Items))

		countLeaf := 0
		traverseRtreeAndTestIfBoundingBoxCorrect(rt.Root, &countLeaf, t)
	})

}

func randomLatLon(minLat, maxLat, minLon, maxLon float64) (float64, float64) {
	rand.Seed(uint64(time.Now().UnixNano()))
	lat := minLat + rand.Float64()*(maxLat-minLat)
	lon := minLon + rand.Float64()*(maxLon-minLon)
	return lat, lon
}

func TestNNearestNeighbors(t *testing.T) {
	t.Run("Test N Nearest Neighbors kota surakarta", func(t *testing.T) {
		itemsData := []OSMObject{
			{
				ID:  7,
				Lat: -7.546392935195944,
				Lon: 110.77718220472673,
			},
			{
				ID:  6,
				Lat: -7.5559986670115675,
				Lon: 110.79466621171177,
			},
			{
				ID:  5,
				Lat: -7.555869730414206,
				Lon: 110.80500875243253,
			},
			{
				ID:  4,
				Lat: -7.571289544570394,
				Lon: 110.8301500772816,
			},
			{
				ID:  3,
				Lat: -7.7886707815273155,
				Lon: 110.361625035987,
			}, {
				ID:  2,
				Lat: -7.8082872068169475,
				Lon: 110.35793427899466,
			},
			{
				ID:  1,
				Lat: -7.759889166547908,
				Lon: 110.36689459108496,
			},
			{
				ID:  0,
				Lat: -7.550561079106621,
				Lon: 110.7837156929654,
			},
		}

		for i := 8; i < 500; i++ {
			lat, lon := randomLatLon(-6.107481038495567, -5.995288834299442, 106.13128828884481, 107.0509652831274)
			itemsData = append(itemsData, OSMObject{
				ID:  i,
				Lat: lat,
				Lon: lon,
			})
		}

		rt := NewRtree(25, 50, 2)
		for _, item := range itemsData {
			rt.InsertLeaf(item.GetBound(), item)
		}

		myLocation := Point{-7.548263971398246, 110.78226484631368}
		results := rt.FastNNearestNeighbors(5, myLocation)

		assert.Equal(t, 5, len(results))
		assert.Equal(t, 0, results[0].Leaf.ID)
		assert.Equal(t, 7, results[1].Leaf.ID)
		assert.Equal(t, 6, results[2].Leaf.ID)
		assert.Equal(t, 5, results[3].Leaf.ID)
		assert.Equal(t, 4, results[4].Leaf.ID)
	})
}

func TestNearestNeighbor(t *testing.T) {
	t.Run("Test N Nearest Neighbors kota surakarta", func(t *testing.T) {
		itemsData := []OSMObject{
			{
				ID:  7,
				Lat: -7.546392935195944,
				Lon: 110.77718220472673,
			},
			{
				ID:  6,
				Lat: -7.5559986670115675,
				Lon: 110.79466621171177,
			},
			{
				ID:  5,
				Lat: -7.555869730414206,
				Lon: 110.80500875243253,
			},
			{
				ID:  4,
				Lat: -7.571289544570394,
				Lon: 110.8301500772816,
			},
			{
				ID:  3,
				Lat: -7.7886707815273155,
				Lon: 110.361625035987,
			}, {
				ID:  2,
				Lat: -7.8082872068169475,
				Lon: 110.35793427899466,
			},
			{
				ID:  1,
				Lat: -7.759889166547908,
				Lon: 110.36689459108496,
			},
			{
				ID:  1000,
				Lat: -7.550561079106621,
				Lon: 110.7837156929654,
			},
			{
				ID:  1001,
				Lat: -7.755002453207869,
				Lon: 110.37712514761436,
			},
		}

		for i := 8; i < 500; i++ {
			lat, lon := randomLatLon(-6.107481038495567, -5.995288834299442, 106.13128828884481, 107.0509652831274)
			itemsData = append(itemsData, OSMObject{
				ID:  i,
				Lat: lat,
				Lon: lon,
			})
		}

		rt := NewRtree(25, 50, 2)
		for _, item := range itemsData {
			rt.InsertLeaf(item.GetBound(), item)
		}

		myLocation := Point{-7.760335932763678, 110.37671195413539}

		result := rt.ImprovedNearestNeighbor(myLocation)
		assert.Equal(t, 1001, result.Leaf.ID)

	})
}

func BenchmarkNNearestNeighbors(b *testing.B) {
	itemsData := []OSMObject{}

	for i := 0; i < 100000; i++ {

		lat, lon := randomLatLon(-6.809629930307937, -6.896578040216839, 105.99351536809907, 112.60245825180131)
		itemsData = append(itemsData, OSMObject{
			ID:  i,
			Lat: lat,
			Lon: lon,
		})
	}

	rt := NewRtree(25, 50, 2)
	for _, item := range itemsData {
		rt.InsertLeaf(item.GetBound(), item)
	}

	myLocation := Point{-7.548263971398246, 110.78226484631368}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rt.FastNNearestNeighbors(5, myLocation)
	}

}

func BenchmarkInsert(b *testing.B) {
	itemsData := []OSMObject{}

	for i := 0; i < 100000; i++ {

		lat, lon := randomLatLon(-6.809629930307937, -6.896578040216839, 105.99351536809907, 112.60245825180131)
		itemsData = append(itemsData, OSMObject{
			ID:  i,
			Lat: lat,
			Lon: lon,
		})
	}

	rt := NewRtree(25, 50, 2)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		randInt := rand.Intn(100000)
		item := itemsData[randInt]
		rt.InsertLeaf(item.GetBound(), item)
	}

}

func BenchmarkImprovedNearestNeighbor(b *testing.B) {
	itemsData := []OSMObject{}

	for i := 0; i < 100000; i++ {

		lat, lon := randomLatLon(-6.809629930307937, -6.896578040216839, 105.99351536809907, 112.60245825180131)
		itemsData = append(itemsData, OSMObject{
			ID:  i,
			Lat: lat,
			Lon: lon,
		})
	}

	rt := NewRtree(25, 50, 2)
	for _, item := range itemsData {
		rt.InsertLeaf(item.GetBound(), item)
	}
	myLocation := Point{-7.548263971398246, 110.78226484631368}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rt.ImprovedNearestNeighbor(myLocation)
	}
}
