package datastructure

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"os"
	"sort"
)

type RtreeBoundingBox struct {
	// number of dimensions
	Dim int
	// Edges[i][0] = low value, Edges[i][1] = high value
	// i = 0,...,Dim
	Edges [][2]float64
}

func NewRtreeBoundingBox(dim int, minVal []float64, maxVal []float64) RtreeBoundingBox {
	b := RtreeBoundingBox{Dim: dim, Edges: make([][2]float64, dim)}
	for axis := 0; axis < dim; axis++ {
		b.Edges[axis] = [2]float64{minVal[axis], maxVal[axis]}
	}

	return b
}

func boundingBox(b RtreeBoundingBox, bb RtreeBoundingBox) RtreeBoundingBox {
	newBound := NewRtreeBoundingBox(b.Dim, make([]float64, b.Dim), make([]float64, b.Dim))

	for axis := 0; axis < b.Dim; axis++ {
		if b.Edges[axis][0] <= bb.Edges[axis][0] {
			newBound.Edges[axis][0] = b.Edges[axis][0]
		} else {
			newBound.Edges[axis][0] = bb.Edges[axis][0]
		}

		if b.Edges[axis][1] >= bb.Edges[axis][1] {
			newBound.Edges[axis][1] = b.Edges[axis][1]
		} else {
			newBound.Edges[axis][1] = bb.Edges[axis][1]
		}
	}

	return newBound
}

// area calculates the area (in N dimensions) of a bounding box.
func area(b RtreeBoundingBox) float64 {
	area := 1.0
	for axis := 0; axis < b.Dim; axis++ {
		area *= b.Edges[axis][1] - b.Edges[axis][0]
	}
	return area
}

// overlaps checks if two bounding boxes overlap.
func overlaps(b RtreeBoundingBox, bb RtreeBoundingBox) bool {
	for axis := 0; axis < b.Dim; axis++ {
		if !(b.Edges[axis][0] < bb.Edges[axis][1]) || !(bb.Edges[axis][0] < b.Edges[axis][1]) {
			/*


				____________________	______________________
				|	b				|   |					   |
				|					|   |			bb		   |
				|	   				|   |					   |
				____________________    |  ____________________

				or

				____________________	______________________
				|	bb				|   |					   |
				|					|   |			b		   |
				|	   				|   |					   |
				____________________    |  ____________________


			*/
			return false
		}
	}

	return true
}

// isBBSame determines if two bounding boxes are identical
func (b *RtreeBoundingBox) isBBSame(bb RtreeBoundingBox) bool {
	for axis := 0; axis < b.Dim; axis++ {
		if b.Edges[axis][0] != bb.Edges[axis][0] || b.Edges[axis][1] != bb.Edges[axis][1] {
			return false
		}
	}

	return true
}

type BoundedItem interface {
	GetBound() RtreeBoundingBox
	isLeafNode() bool
	IsData() bool
}

// rtree node. can be either a leaf node or a internal node or leafData.
type RtreeNode struct {
	// entries. can be either a leaf node or a  internal node.
	// leafNode has items in the form of a list of RtreeLeaf
	Items  []*RtreeNode
	Parent *RtreeNode

	Bound RtreeBoundingBox
	// isLeaf. true if  this node is a leafNode.
	IsLeaf bool

	Leaf OSMObject // if this node is a leafData
}

// isLeaf. true if this node is a leafNode.
func (node *RtreeNode) isLeafNode() bool {
	return node.IsLeaf
}

func (node *RtreeNode) GetBound() RtreeBoundingBox {
	return node.Bound
}

func (node *RtreeNode) ComputeBB() RtreeBoundingBox {
	if len(node.Items) == 1 {
		return node.Items[0].GetBound()
	}
	bb := boundingBox(node.Items[0].GetBound(), node.Items[1].GetBound())
	for i := 2; i < len(node.Items); i++ {
		bb = boundingBox(bb, node.Items[i].GetBound())
	}
	return bb
}

func (node *RtreeNode) IsData() bool {
	return false
}

type Rtree struct {
	Root          *RtreeNode
	Size          int
	MinChildItems int
	MaxChildItems int
	Dimensions    int
	Height        int
}

func NewRtree(minChildItems, maxChildItems, dimensions int) *Rtree {

	return &Rtree{
		Root:          nil,
		Size:          0,
		Height:        0,
		MinChildItems: minChildItems,
		MaxChildItems: maxChildItems,
		Dimensions:    dimensions,
	}

}

func (rt *Rtree) InsertR(bound RtreeBoundingBox, leaf OSMObject) {
	if rt.Root == nil {
		rt.Root = &RtreeNode{
			IsLeaf: true,
			Items:  make([]*RtreeNode, 0, rt.MaxChildItems),
		}
	}
	newLeaf := &RtreeNode{}
	newLeaf.Bound = bound
	newLeaf.Leaf = leaf

	leafNode := rt.chooseLeaf(rt.Root, leaf.GetBound())
	leafNode.Items = append(leafNode.Items, newLeaf)

	newLeaf.Parent = leafNode

	rt.Size++

	var l, ll *RtreeNode
	l = leafNode
	if len(leafNode.Items) > rt.MaxChildItems {
		l, ll = rt.splitNode(leafNode)
	}

	p, pp := rt.adjustTree(l, ll)
	if pp != nil {
		// 14. [Grow tree taller.] If node split propagation caused the root to split,
		// create a new root whose children are
		// the two resulting nodes.
		rt.Root = &RtreeNode{}
		pp.Bound = pp.ComputeBB()

		rt.Root.Items = []*RtreeNode{p, pp}
		p.Parent = rt.Root
		pp.Parent = rt.Root
		rt.Height++

		rt.Root.Bound = rt.Root.ComputeBB()
	}
}

func (rt *Rtree) adjustTree(l, ll *RtreeNode) (*RtreeNode, *RtreeNode) {
	//ATI. [Initialize.] Set N=L. If L was split
	// previously, set NN to be the resulting
	// second node.
	n := l
	var nn *RtreeNode
	if ll != nil {
		nn = ll
	}

	if n == rt.Root {
		n.Bound = n.ComputeBB()
		//AT2. [Check if done.] If N is the root, stop.
		return n, nn
	}
	//AT3. [Adjust covering rectangle in parent
	// entry.] Let P be the parent node of
	// N, and let EN be N's entry in P.
	// Adjust En.I s o that it tightly encloses
	// all entry rectangles in N
	p := n.Parent
	en := n.Items[0]

	for i := 0; i < len(p.Items); i++ {
		if p.Items[i] == n {
			en = n
		}
	}

	en.Bound = n.ComputeBB()

	//AT4. [Propagate node split upward.] If N
	// has a partner NN resulting from an
	// earlier split, create a new entry ENN
	// with ENN.p pointing to NN and Enn.I
	// enclosing all rectangles in NN. Add
	//Enn to P if there is room Otherwise,
	//invoke SplitNode to produce P and
	// PP containing Em and all P ’s old
	// entries.
	//AT5. [Move up to next level.] Set N=P and
	//set NN-PP if a split occurred.
	//Repeat from AT2.

	if nn != nil {
		enn := nn
		enn.Bound = nn.ComputeBB()

		p.Items = append(p.Items, enn)
		if len(p.Items) > rt.MaxChildItems {
			return rt.adjustTree(rt.splitNode(p))
		}
	}

	return rt.adjustTree(p, nil)
}

func (rt *Rtree) insertSort(dists []float64, dist float64, sorted []*RtreeNode,
	obj *RtreeNode, max int) ([]*RtreeNode, []float64) {

	idx := sort.SearchFloat64s(dists, dist)
	for idx < len(sorted) && dist >= dists[idx] {
		idx++
	}

	if idx >= max {
		return sorted, dists
	}

	if len(sorted) < max {
		dists = append(dists, 0)
		sorted = append(sorted, &RtreeNode{})
	}

	copy(dists, dists[:idx])
	copy(dists[idx+1:], dists[idx:len(dists)-1])
	dists[idx] = dist

	copy(sorted, sorted[:idx])
	copy(sorted[idx+1:], sorted[idx:len(sorted)-1])
	sorted[idx] = obj

	return sorted, dists
}

func (rt *Rtree) splitNode(l *RtreeNode) (*RtreeNode, *RtreeNode) {
	//QSl. [Pick first entry for each group.]
	// Apply Algorithm PickSeeds to choose
	// two entries to be the first elements
	// of the groups. Assign each to a group
	firstEntryGroupOne, firstEntryGroupTwo := rt.linearPickSeeds(l)

	remaining := make([]*RtreeNode, 0, len(l.Items)-2)
	for i := 0; i < len(l.Items); i++ {
		if l.Items[i] != firstEntryGroupOne && l.Items[i] != firstEntryGroupTwo {
			remaining = append(remaining, l.Items[i])
		}
	}

	groupOne := l
	groupOne.Items = []*RtreeNode{firstEntryGroupOne}
	groupOne.Parent = l.Parent
	firstEntryGroupOne.Parent = groupOne

	groupTwo := &RtreeNode{
		Parent: l.Parent,
		Items:  []*RtreeNode{firstEntryGroupTwo},
		IsLeaf: l.IsLeaf,
	}
	firstEntryGroupTwo.Parent = groupTwo

	//QS2. [Check if done.] If all entries have
	// been assigned, stop. If one group has
	// so few entries that all the rest must
	// be assigned to it in order for it to
	// have the minimum number m , assign
	// them and stop.
	for len(remaining) > 0 {
		// QS3. [Select entry to assign.] Invoke Algorithm PickNext to choose the next
		// entry to assign. Add it to the group
		// whose covering rectangle will have to
		// be enlarged least to accommodate it.
		// Resolve ties by adding the entry to
		// the group with smaller area, then to
		// the one with fewer entries, then to
		// either. Repeat from QS2.

		nextEntryIdx := rt.pickNext(groupOne, groupTwo, remaining)
		groupOneBB := groupOne.ComputeBB()
		groupTwoBB := groupTwo.ComputeBB()

		bbGroupOne := boundingBox(groupOneBB, remaining[nextEntryIdx].GetBound())
		enlargementOne := area(bbGroupOne) - area(groupOneBB)

		bbGroupTwo := boundingBox(groupTwoBB, remaining[nextEntryIdx].GetBound())
		enlargementTwo := area(bbGroupTwo) - area(groupTwoBB)

		if len(groupOne.Items)+len(l.Items) <= rt.MinChildItems {
			groupOne.Items = append(groupOne.Items, remaining[nextEntryIdx])
			remaining[nextEntryIdx].Parent = groupOne
		} else if len(groupTwo.Items)+len(l.Items) <= rt.MinChildItems {
			groupTwo.Items = append(groupTwo.Items, remaining[nextEntryIdx])
			remaining[nextEntryIdx].Parent = groupTwo
		} else {
			if enlargementOne < enlargementTwo ||
				(enlargementOne == enlargementTwo && area(bbGroupOne) < area(bbGroupTwo)) ||
				(enlargementOne == enlargementTwo && area(bbGroupOne) == area(bbGroupTwo) &&
					len(groupOne.Items) < len(groupTwo.Items)) {
				groupOne.Items = append(groupOne.Items, remaining[nextEntryIdx])
				remaining[nextEntryIdx].Parent = groupOne
			} else if enlargementOne > enlargementTwo ||
				(enlargementOne == enlargementTwo && area(bbGroupOne) > area(bbGroupTwo)) ||
				(enlargementOne == enlargementTwo && area(bbGroupOne) == area(bbGroupTwo) &&
					len(groupOne.Items) > len(groupTwo.Items)) {
				groupTwo.Items = append(groupTwo.Items, remaining[nextEntryIdx])
				remaining[nextEntryIdx].Parent = groupTwo
			}
		}

		remaining = append(remaining[:nextEntryIdx], remaining[nextEntryIdx+1:]...)
	}

	return groupOne, groupTwo
}

func (rt *Rtree) pickNext(groupOne, groupTwo *RtreeNode, remaining []*RtreeNode) int {
	/*
		PN1. [Determine cost of putting each
		entry in each group.] For each entry
		E not yet in a group, calculate d1=
		the area increase required in the
		covering rectangle of Group 1 to
		include E.I. Calculate d2 similarly
		for Group 2

		PN2. [Find entry with greatest preference
		for one group.] Choose any entry
		with the maximum difference
		between d 1 and d 2
	*/
	chosen := 0
	maxDiff := math.Inf(-1)
	groupOneBB := groupOne.GetBound()
	groupTwoBB := groupTwo.GetBound()
	for i := 0; i < len(remaining); i++ {
		enBBGroupOne := boundingBox(groupOneBB, remaining[i].GetBound())
		d1 := area(enBBGroupOne) - area(groupOneBB)

		enBBGroupTwo := boundingBox(groupTwoBB, remaining[i].GetBound())
		d2 := area(enBBGroupTwo) - area(groupTwoBB)

		d := math.Abs(d1 - d2)

		if d > maxDiff {
			chosen = i
			maxDiff = d
		}
	}
	return chosen
}

/*
LPSl.[Find extreme rectangles along all
dimensions.] Along each dimension,
find the entry whose rectangle has
the highest low side, and the one
with the lowest high side. Record the
separation.

LPS2. [Adjust for shape of the rectangle
cluster.] Normalize the separations
by dividing by the width of the entire
set along the corresponding dimension.

LPS3. [Select the most extreme pair.]
Choose the pair with the greatest
normalized separation along any
dimension.
*/
func (rt *Rtree) linearPickSeeds(l *RtreeNode) (*RtreeNode, *RtreeNode) {

	entryOne := l.Items[0]
	entryTwo := l.Items[1]

	greatestNormalizedSeparation := math.Inf(-1)
	for axis := 0; axis < rt.Dimensions; axis++ {
		distsLowSide := make([]float64, 0, len(l.Items))
		lowSide := make([]*RtreeNode, 0, len(l.Items))

		highSide := make([]*RtreeNode, 0, len(l.Items))
		distsHighSide := make([]float64, 0, len(l.Items))

		for i := 0; i < len(l.Items); i++ {
			lowSideEdge := l.Items[i].Bound.Edges[axis][0]
			lowSide, distsLowSide = rt.insertSort(distsLowSide, lowSideEdge, lowSide, l.Items[i], len(l.Items))

			highSideEdge := l.Items[i].Bound.Edges[axis][1]
			highSide, distsHighSide = rt.insertSort(distsHighSide, highSideEdge, highSide, l.Items[i], len(l.Items))
		}

		highestLowSide := distsLowSide[len(lowSide)-1]
		lowestHighSide := distsHighSide[0]

		lWidth := highestLowSide - lowestHighSide

		widthAlongDimension := distsLowSide[0] - distsHighSide[len(distsHighSide)-1]

		if lWidth/widthAlongDimension > greatestNormalizedSeparation {
			greatestNormalizedSeparation = lWidth / widthAlongDimension
			entryOne = lowSide[len(lowSide)-1]
			entryTwo = highSide[0]
		}
	}

	return entryOne, entryTwo
}

/*
CLl. [Initialize.] Set N to be the root
node.
CL2. [Leaf check.] If N is a leaf, return N.
CL3. [Choose subtree.] If Af is not a leaf,
let F be the entry in N whose rectangle F.I needs least enlargement to
include E.I. Resolve ties by choosing
the entry with the rectangle of smallest area.
CL4. [Descend until a leaf is reached.] Set
N to be the child node pointed to by
F.p and repeat from CL2.
*/
func (rt *Rtree) chooseLeaf(node *RtreeNode, bound RtreeBoundingBox) *RtreeNode {

	if node.isLeafNode() {
		return node
	}
	var chosen *RtreeNode

	minAreaEnlargement := math.MaxFloat64
	idxEntryWithMinAreaEnlargement := 0
	for i, item := range node.Items {
		itembb := item.GetBound()

		bb := boundingBox(itembb, bound)

		enlargement := area(bb) - area(itembb)
		if enlargement < minAreaEnlargement ||
			(enlargement == minAreaEnlargement &&
				area(bb) < area(node.Items[idxEntryWithMinAreaEnlargement].GetBound())) {
			minAreaEnlargement = enlargement
			idxEntryWithMinAreaEnlargement = i
		}
	}

	chosen = node.Items[idxEntryWithMinAreaEnlargement]

	return rt.chooseLeaf(chosen, bound)
}

func (rt *Rtree) Search(bound RtreeBoundingBox) []RtreeNode {
	results := []RtreeNode{}
	return rt.search(rt.Root, bound, results)
}

func (rt *Rtree) search(node *RtreeNode, bound RtreeBoundingBox,
	results []RtreeNode) []RtreeNode {
	for _, e := range node.Items {

		if !overlaps(e.GetBound(), bound) {
			continue
		}

		if !node.isLeafNode() {
			// S1. [Search subtrees.] If T is not a leaf,
			// check each entry E to determine
			// whether E.I overlaps S. For all overlapping entries, invoke Search on the tree
			// whose root node is pointed to by E.p
			results = rt.search(e, bound, results)
			continue
		}

		if overlaps(e.GetBound(), bound) {
			// S2. [Search leaf node.] If T is a leaf, check
			// all entries E to determine whether E.I
			// overlaps S. If so, E is a qualifying
			// record
			results = append(results, *e)

		}
	}
	return results
}

type Point struct {
	Lat float64
	Lon float64
}

// minDist computes the square of the distance from a point to a rectangle. If the point is contained in the rectangle then the distance is zero.
func (p Point) minDist(r RtreeBoundingBox) float64 {

	// Edges[0] = {minLat, maxLat}
	// Edges[1] = {minLon, maxLon}
	sum := 0.0
	rLat, rLon := 0.0, 0.0
	if p.Lat < r.Edges[0][0] {
		rLat = r.Edges[0][0]
	} else if p.Lat > r.Edges[0][1] {
		rLat = r.Edges[0][1]
	} else {
		rLat = p.Lat
	}

	if p.Lon < r.Edges[1][0] {
		rLon = r.Edges[1][0]
	} else if p.Lon > r.Edges[1][1] {
		rLon = r.Edges[1][1]
	} else {
		rLon = p.Lon
	}

	sum += euclideanDistance(p.Lat, p.Lon, rLat, rLon)

	return sum
}

type OSMObject struct {
	ID  int
	Lat float64
	Lon float64
}

func (o *OSMObject) GetBound() RtreeBoundingBox {
	return NewRtreeBoundingBox(2, []float64{o.Lat - 0.0001, o.Lon - 0.0001}, []float64{o.Lat + 0.0001, o.Lon + 0.0001})
}

func (o *OSMObject) isLeafNode() bool {
	return false
}

func (o *OSMObject) IsData() bool {
	return true
}

func (rt *Rtree) NearestNeighboursPQ(k int, p Point) []OSMObject {
	nearestLists := make([]OSMObject, 0, k)

	callback := func(n OSMObject) bool {
		nearestLists = append(nearestLists, n)
		return len(nearestLists) < k
	}

	rt.nearestNeigboursPQ(p, callback)

	return nearestLists
}
// https://dl.acm.org/doi/pdf/10.1145/320248.320255
func (rt *Rtree) nearestNeigboursPQ(p Point, callback func(OSMObject) bool) {
	pq := NewMinHeap()
	pq.Insert(NewPriorityQueueNodeRtree2(0, rt.Root))

	for pq.Size() > 0 {

		element, ok := pq.ExtractMin()
		if !ok {
			return
		}
		if element.Item.IsData() {
			first, _ := pq.GetMin()
			for element == first {
				first, _ = pq.ExtractMin()
			}
			if !callback(*element.Item.(*OSMObject)) {
				return
			}
		} else if element.Item.isLeafNode() {
			distToElement := p.minDist(element.Item.GetBound())

			for _, item := range element.Item.(*RtreeNode).Items {
				distToObject := euclideanDistance(p.Lat, p.Lon, item.Leaf.Lat, item.Leaf.Lon)

				if distToObject > distToElement {
					pq.Insert(NewPriorityQueueNodeRtree2(distToObject, &item.Leaf))
				}
			}
		} else {
			for _, item := range element.Item.(*RtreeNode).Items {

				pq.Insert(NewPriorityQueueNodeRtree2(p.minDist(item.GetBound()), item))
			}
		}
	}
}

func (rt *Rtree) ImprovedNearestNeighbor(p Point) OSMObject {

	nnDistTemp := math.Inf(1)

	nearest, _ := rt.nearestNeighbor(p, nnDistTemp)
	return nearest
}

// https://rutgers-db.github.io/cs541-fall19/slides/notes4.pdf
func (rt *Rtree) nearestNeighbor(p Point, nnDistTemp float64) (OSMObject, float64) {
	nearest := OSMObject{}
	pq := NewMinHeap()

	pq.Insert(NewPriorityQueueNodeRtree2(p.minDist(rt.Root.GetBound()), rt.Root))

	bestDist := math.Inf(1)
	smallestMinDist, _ := pq.GetMin()
	for bestDist > smallestMinDist.Rank {
		currR, ok := pq.ExtractMin()
		if !ok {
			break
		}
		for _, item := range currR.Item.(*RtreeNode).Items {
			if !item.isLeafNode() {
				pq.Insert(NewPriorityQueueNodeRtree2(p.minDist(item.GetBound()), item))
			} else {
				for _, leafData := range item.Items {
					dist := euclideanDistance(p.Lat, p.Lon, leafData.Leaf.Lat, leafData.Leaf.Lon)
					if dist < bestDist {
						bestDist = dist
						nearest = leafData.Leaf
					}
				}
			}
		}
		smallestMinDist, ok = pq.GetMin()
		if !ok {
			break
		}
	}

	return nearest, nnDistTemp
}

func SerializeRtreeData(workingDir string, outputDir string, items []OSMObject) error {

	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(items)
	if err != nil {
		return err
	}

	var rtreeFile *os.File
	if workingDir != "/" {
		rtreeFile, err = os.OpenFile(workingDir+"/"+outputDir+"/"+"rtree.dat", os.O_RDWR|os.O_CREATE, 0700)
		if err != nil {
			return err
		}
	} else {
		rtreeFile, err = os.OpenFile(outputDir+"/"+"rtree.dat", os.O_RDWR|os.O_CREATE, 0700)
		if err != nil {
			return err
		}
	}
	_, err = rtreeFile.Write(buf.Bytes())

	return err
}

func (rt *Rtree) Deserialize(workingDir string, outputDir string) error {

	var rtreeFile *os.File
	var err error
	if workingDir != "/" {
		rtreeFile, err = os.Open(workingDir + "/" + outputDir + "/" + "rtree.dat")
		if err != nil {
			return fmt.Errorf("error opening file: %v", err)
		}
	} else {
		rtreeFile, err = os.Open(outputDir + "/" + "rtree.dat")
		if err != nil {
			return fmt.Errorf("error opening file: %v", err)
		}
	}

	stat, err := os.Stat(rtreeFile.Name())
	if err != nil {
		return fmt.Errorf("error when getting metadata file stat: %w", err)
	}

	buf := make([]byte, stat.Size()*2)

	_, err = rtreeFile.Read(buf)
	if err != nil {
		return err
	}

	gobDec := gob.NewDecoder(bytes.NewBuffer(buf))

	items := []OSMObject{}
	err = gobDec.Decode(&items)
	if err != nil {
		return err
	}

	for _, item := range items {
		rt.InsertR(item.GetBound(), item)
	}

	return nil
}
