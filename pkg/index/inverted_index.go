package index

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"osm-search/pkg"
	"osm-search/pkg/datastructure"
	"osm-search/pkg/geo"
	"sort"
	"strconv"
	"strings"

	"github.com/RadhiFadlillah/go-sastrawi"
	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

type SpellCorrectorI interface {
	Preprocessdata(tokenizedDocs [][]string)
	GetWordCandidates(mispelledWord string, editDistance int) ([]int, error)
	GetCorrectQueryCandidates(allPossibleQueryTerms [][]int) [][]int
	GetCorrectSpellingSuggestion(allCorrectQueryCandidates [][]int, originalQueryTermIDs []int) ([]int, error)
	GetMatchedWordBasedOnPrefix(prefixWord string) ([]int, error)
	GetMatchedWordsAutocomplete(allQueryCandidates [][]int, originalQueryTerms []int) ([][]int, error)
}

type DocumentStoreI interface {
	WriteDocs(docs []datastructure.Node)
}

type BboltDBI interface {
	SaveDocs(nodes []datastructure.Node) error
}

// https://nlp.stanford.edu/IR-book/pdf/04const.pdf (4.3 Single-pass in-memory indexing)
type DynamicIndex struct {
	TermIDMap                 pkg.IDMap
	WorkingDir                string
	IntermediateIndices       []string
	InMemoryIndices           map[int][]int
	MaxDynamicPostingListSize int
	DocWordCount              map[int]int
	OutputDir                 string
	DocsCount                 int
	SpellCorrectorBuilder     SpellCorrectorI
	IndexedData               IndexedData
	DocumentStore             BboltDBI //DocumentStoreI
}

type IndexedData struct {
	Ways     []geo.OSMWay
	Nodes    []geo.OSMNode
	Ctr      geo.NodeMapContainer
	TagIDMap pkg.IDMap
}

func NewIndexedData(ways []geo.OSMWay, nodes []geo.OSMNode, ctr geo.NodeMapContainer, tagIDMap pkg.IDMap) IndexedData {
	return IndexedData{
		Ways:     ways,
		Nodes:    nodes,
		Ctr:      ctr,
		TagIDMap: tagIDMap,
	}
}

type InvertedIDXDB interface {
	SaveDocs(nodes []datastructure.Node) error
}

func NewDynamicIndex(outputDir string, maxPostingListSize int,
	server bool, spell SpellCorrectorI, indexedData IndexedData, boltDB BboltDBI) (*DynamicIndex, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return &DynamicIndex{}, err
	}
	idx := &DynamicIndex{
		TermIDMap:                 pkg.NewIDMap(),
		IntermediateIndices:       []string{},
		WorkingDir:                pwd,
		InMemoryIndices:           make(map[int][]int),
		MaxDynamicPostingListSize: maxPostingListSize,
		DocWordCount:              make(map[int]int),
		OutputDir:                 outputDir,
		DocsCount:                 0,
		SpellCorrectorBuilder:     spell,
		IndexedData:               indexedData,
		DocumentStore:             boltDB,
	}
	if server {
		err := idx.LoadMeta()
		if err != nil {
			return nil, err
		}
	}

	return idx, nil
}

func (Idx *DynamicIndex) SpimiBatchIndex() error {
	searchNodes := []datastructure.Node{}
	nodeIDX := 0
	fmt.Println("")
	bar := progressbar.NewOptions(5,
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetDescription("[cyan][2/2]Indexing osm objects..."),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
	fmt.Println("")
	bar.Add(1)
	fmt.Println("")
	block := 0

	nodeBoundingBox := make(map[string]geo.BoundingBox)

	for _, way := range Idx.IndexedData.Ways {
		lat := make([]float64, len(way.NodeIDs))
		lon := make([]float64, len(way.NodeIDs))
		for i := 0; i < len(way.NodeIDs); i++ {
			node := way.NodeIDs[i]
			nodeLat := Idx.IndexedData.Ctr.GetNode(node).Lat
			nodeLon := Idx.IndexedData.Ctr.GetNode(node).Lon
			lat[i] = nodeLat
			lon[i] = nodeLon
		}

		centerLat, centerLon, err := geo.CenterOfPolygonLatLon(lat, lon)
		if err != nil {
			return err
		}
		tagStringMap := make(map[string]string)
		for k, v := range way.TagMap {
			tagStringMap[Idx.IndexedData.TagIDMap.GetStr(k)] = Idx.IndexedData.TagIDMap.GetStr(v)

		}

		name, address, tipe, city := geo.GetNameAddressTypeFromOSMWay(tagStringMap)

		if IsWayDuplicateCheck(strings.ToLower(name), lat, lon, nodeBoundingBox) {
			// cek duplikat kalo sebelumnya ada way dengan nama sama dan posisi sama dengan way ini.
			continue
		}

		nodeBoundingBox[strings.ToLower(name)] = geo.NewBoundingBox(lat, lon)

		searchNodes = append(searchNodes, datastructure.NewNode(nodeIDX, name, centerLat,
			centerLon, address, tipe, city))
		nodeIDX++

		if len(searchNodes) == 200000 {
			err := Idx.SpimiInvert(searchNodes, &block)
			if err != nil {
				return err
			}
		
			
			err = Idx.DocumentStore.SaveDocs(searchNodes)
			if err != nil {
				return err
			}
			searchNodes = []datastructure.Node{}
		}
	}
	bar.Add(1)

	for _, node := range Idx.IndexedData.Nodes {
		tagStringMap := make(map[string]string)
		for k, v := range node.TagMap {
			tagStringMap[Idx.IndexedData.TagIDMap.GetStr(k)] = Idx.IndexedData.TagIDMap.GetStr(v)
		}
		name, address, tipe, city := geo.GetNameAddressTypeFromOSNode(tagStringMap)
		if name == "" {
			continue
		}

		if IsNodeDuplicateCheck(strings.ToLower(name), node.Lat, node.Lon, nodeBoundingBox) {
			// cek duplikat kalo sebelumnya ada way dengan nama sama dan posisi sama dengan node ini. gak usah set bounding box buat node.
			continue
		}

		searchNodes = append(searchNodes, datastructure.NewNode(nodeIDX, name, node.Lat,
			node.Lon, address, tipe, city))
		nodeIDX++
		if len(searchNodes) == 200000 {
			err := Idx.SpimiInvert(searchNodes, &block)
			if err != nil {
				return err
			}
			err = Idx.DocumentStore.SaveDocs(searchNodes)
			if err != nil {
				return err
			}
			searchNodes = []datastructure.Node{}
		}
	}

	Idx.DocsCount = nodeIDX

	bar.Add(1)
	err := Idx.SpimiInvert(searchNodes, &block)
	if err != nil {
		return err
	}

	err = Idx.DocumentStore.SaveDocs(searchNodes)
	if err != nil {
		return err
	}
	bar.Add(1)

	mergedIndex := NewInvertedIndex("merged_index", Idx.OutputDir, Idx.WorkingDir)
	indices := []*InvertedIndex{}
	for _, indexID := range Idx.IntermediateIndices {
		index := NewInvertedIndex(indexID, Idx.OutputDir, Idx.WorkingDir)
		err := index.OpenReader()
		if err != nil {
			return err
		}
		indices = append(indices, index)
	}
	mergedIndex.OpenWriter()

	err = Idx.Merge(indices, mergedIndex)
	if err != nil {
		return err
	}
	for _, index := range indices {
		err := index.Close()
		if err != nil {
			return err
		}
	}
	err = mergedIndex.Close()
	if err != nil {
		return err
	}
	bar.Add(1)
	return nil
}

func IsWayDuplicateCheck(name string, lats, lons []float64, nodeBoundingBox map[string]geo.BoundingBox) bool {
	prevBB, ok := nodeBoundingBox[name]

	if !ok {
		return false
	}
	contain := prevBB.PointsContains(lats, lons)

	if !contain {
		// perbesar bounding box nya karena namanya sama tapi mungkin bb sebelumnya lebih kecil & gak contain bb ini.
		nodeBoundingBox[name] = geo.NewBoundingBox(lats, lons)
	}

	currWayBB := geo.NewBoundingBox(lats, lons)
	inverseContain := currWayBB.PointsContains(prevBB.GetMin(), prevBB.GetMax()) // cek sebaliknya (cuur osm way Bounding Box contain previous same name bounding box)
	return contain || inverseContain
}

func IsNodeDuplicateCheck(name string, lats, lon float64, nodeBoundingBox map[string]geo.BoundingBox) bool {
	prevBB, ok := nodeBoundingBox[name]
	if !ok {
		return false
	}
	contain := prevBB.Contains(lats, lon)
	return contain
}

func (Idx *DynamicIndex) SpimiIndex(nodes []datastructure.Node) error {
	block := 0
	Idx.SpimiInvert(nodes, &block)

	mergedIndex := NewInvertedIndex("merged_index", Idx.OutputDir, Idx.WorkingDir)
	indices := []*InvertedIndex{}
	for _, indexID := range Idx.IntermediateIndices {
		index := NewInvertedIndex(indexID, Idx.OutputDir, Idx.WorkingDir)
		index.OpenReader()
		indices = append(indices, index)
	}
	mergedIndex.OpenWriter()

	err := Idx.Merge(indices, mergedIndex)
	if err != nil {
		return err
	}
	for _, index := range indices {
		index.Close()
	}
	return nil
}

func (Idx *DynamicIndex) Merge(indices []*InvertedIndex, mergedIndex *InvertedIndex) error {
	lastTerm, lastPosting := -1, []int{}
	mergeKArrayIterator := NewMergeKArrayIterator(indices)
	for output, err := range mergeKArrayIterator.mergeKArray() {
		if err != nil {
			return fmt.Errorf("error when merge posting lists: %w", err)
		}
		currTerm, currPostings := output.TermID, output.Postings

		if currTerm != lastTerm {
			if lastTerm != -1 {
				sort.Ints(lastPosting)
				err := mergedIndex.AppendPostingList(lastTerm, lastPosting)
				if err != nil {
					return fmt.Errorf("error when merge posting lists: %w", err)
				}
			}
			lastTerm, lastPosting = currTerm, currPostings
		} else {
			lastPosting = append(lastPosting, currPostings...)
		}

	}

	if lastTerm != -1 {
		sort.Ints(lastPosting)
		err := mergedIndex.AppendPostingList(lastTerm, lastPosting)
		if err != nil {
			return err
		}
	}
	return nil
}

// https://nlp.stanford.edu/IR-book/pdf/04const.pdf (Figure 4.4 Spimi-invert)
func (Idx *DynamicIndex) SpimiInvert(nodes []datastructure.Node, block *int) error {
	postingSize := 0

	termToPostingMap := make(map[int][]int)
	tokenStreams := Idx.SpimiParseOSMNodes(nodes) // [pair of termID and nodeID]

	var postingList []int
	for _, termDocPair := range tokenStreams {

		if len(tokenStreams) == 0 {
			continue
		}
		termID, nodeID := termDocPair[0], termDocPair[1]
		if _, ok := termToPostingMap[termID]; ok {
			postingList = termToPostingMap[termID]
		} else {
			postingList = []int{}
			termToPostingMap[termID] = postingList
		}
		postingList = append(postingList, nodeID)
		termToPostingMap[termID] = postingList
		postingSize += 1

		if postingSize >= Idx.MaxDynamicPostingListSize {
			postingSize = 0
			terms := []int{}
			for termID, _ := range termToPostingMap {
				terms = append(terms, termID)
			}
			sort.Ints(terms)
			indexID := "index_" + strconv.Itoa(*block)
			index := NewInvertedIndex(indexID, Idx.OutputDir, Idx.WorkingDir)
			err := index.OpenWriter()
			if err != nil {
				return err
			}
			Idx.IntermediateIndices = append(Idx.IntermediateIndices, indexID)
			for term := range terms {

				sort.Ints(termToPostingMap[term])
				index.AppendPostingList(term, termToPostingMap[term])
			}
			*block += 1
			termToPostingMap = make(map[int][]int)
			index.Close()
		}
	}

	terms := []int{}
	for termID, _ := range termToPostingMap {
		terms = append(terms, termID)
	}
	sort.Ints(terms)
	indexID := "index_" + strconv.Itoa(*block)
	index := NewInvertedIndex(indexID, Idx.OutputDir, Idx.WorkingDir)
	err := index.OpenWriter()
	if err != nil {
		return err
	}
	Idx.IntermediateIndices = append(Idx.IntermediateIndices, indexID)
	for _, term := range terms {
		sort.Ints(termToPostingMap[term])
		index.AppendPostingList(term, termToPostingMap[term])
	}
	*block += 1
	err = index.Close()
	if err != nil {
		return err
	}
	return nil
}

func (Idx *DynamicIndex) SpimiParseOSMNode(node datastructure.Node) [][]int {
	termDocPairs := [][]int{}

	soup := node.Name + " " + node.Address + " " +
		node.City + " " + node.Tipe
	if soup == "" {
		return termDocPairs
	}

	words := sastrawi.Tokenize(soup)
	Idx.DocWordCount[node.ID] = len(words)
	for _, word := range words {
		tokenizedWord := pkg.Stemmer.Stem(word)
		termID := Idx.TermIDMap.GetID(tokenizedWord)
		pair := []int{termID, node.ID}
		termDocPairs = append(termDocPairs, pair)
	}
	return termDocPairs
}

func (Idx *DynamicIndex) SpimiParseOSMNodes(nodes []datastructure.Node) [][]int {
	termDocPairs := [][]int{}
	for _, node := range nodes {
		termDocPairs = append(termDocPairs, Idx.SpimiParseOSMNode(node)...)
	}
	return termDocPairs
}

func (Idx *DynamicIndex) BuildSpellCorrectorAndNgram() error {
	fmt.Println("")
	bar := progressbar.NewOptions(5,
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetDescription("[cyan][2/2]Building Ngram..."),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
	fmt.Println("")
	bar.Add(1)
	searchNodes := []datastructure.Node{}
	nodeIDX := 0

	nodeBoundingBox := make(map[string]geo.BoundingBox)

	for _, way := range Idx.IndexedData.Ways {
		lat := make([]float64, len(way.NodeIDs))
		lon := make([]float64, len(way.NodeIDs))
		for i := 0; i < len(way.NodeIDs); i++ {
			node := way.NodeIDs[i]
			nodeLat := Idx.IndexedData.Ctr.GetNode(node).Lat
			nodeLon := Idx.IndexedData.Ctr.GetNode(node).Lon
			lat[i] = nodeLat
			lon[i] = nodeLon
		}

		centerLat, centerLon, err := geo.CenterOfPolygonLatLon(lat, lon)
		if err != nil {
			return err
		}
		tagStringMap := make(map[string]string)
		for k, v := range way.TagMap {
			tagStringMap[Idx.IndexedData.TagIDMap.GetStr(k)] = Idx.IndexedData.TagIDMap.GetStr(v)

		}

		name, address, tipe, city := geo.GetNameAddressTypeFromOSMWay(tagStringMap)

		if IsWayDuplicateCheck(strings.ToLower(name), lat, lon, nodeBoundingBox) {
			// cek duplikat kalo sebelumnya ada way dengan nama sama dan posisi sama dengan way ini.
			continue
		}

		nodeBoundingBox[strings.ToLower(name)] = geo.NewBoundingBox(lat, lon)

		searchNodes = append(searchNodes, datastructure.NewNode(nodeIDX, name, centerLat,
			centerLon, address, tipe, city))
		nodeIDX++
	}
	bar.Add(1)

	for _, node := range Idx.IndexedData.Nodes {
		tagStringMap := make(map[string]string)
		for k, v := range node.TagMap {
			tagStringMap[Idx.IndexedData.TagIDMap.GetStr(k)] = Idx.IndexedData.TagIDMap.GetStr(v)
		}
		name, address, tipe, city := geo.GetNameAddressTypeFromOSNode(tagStringMap)
		if name == "" {
			continue
		}

		if IsNodeDuplicateCheck(strings.ToLower(name), node.Lat, node.Lon, nodeBoundingBox) {
			// cek duplikat kalo sebelumnya ada way dengan nama sama dan posisi sama dengan node ini. gak usah set bounding box buat node.
			continue
		}

		searchNodes = append(searchNodes, datastructure.NewNode(nodeIDX, name, node.Lat,
			node.Lon, address, tipe, city))
		nodeIDX++

	}
	bar.Add(1)

	Idx.DocsCount = nodeIDX

	tokenizedDocs := [][]string{}
	for _, node := range searchNodes {

		soup := node.Name + " " + node.Address + " " +
			node.City + " " + node.Tipe

		tokenized := sastrawi.Tokenize(soup)
		stemmedTokens := []string{}
		for _, token := range tokenized {
			stemmedToken := pkg.Stemmer.Stem(token)
			stemmedTokens = append(stemmedTokens, stemmedToken)
		}
		tokenizedDocs = append(tokenizedDocs, stemmedTokens)
	}
	bar.Add(1)
	Idx.SpellCorrectorBuilder.Preprocessdata(tokenizedDocs)
	bar.Add(1)
	fmt.Println("")
	return nil
}

type SpimiIndexMetadata struct {
	TermIDMap    pkg.IDMap
	DocWordCount map[int]int
	DocsCount    int
}

func NewSpimiIndexMetadata(termIDMap pkg.IDMap, docWordCount map[int]int, docsCount int) SpimiIndexMetadata {
	return SpimiIndexMetadata{
		TermIDMap:    termIDMap,
		DocWordCount: docWordCount,
		DocsCount:    docsCount,
	}
}
func (Idx *DynamicIndex) Close() error {
	err := Idx.SaveMeta()
	return err
}

func (Idx *DynamicIndex) SaveMeta() error {
	// save to disk
	SpimiMeta := NewSpimiIndexMetadata(Idx.TermIDMap, Idx.DocWordCount, Idx.DocsCount)
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(SpimiMeta)
	if err != nil {
		return err
	}

	var metadataFile *os.File
	if Idx.WorkingDir != "/" {
		metadataFile, err = os.OpenFile(Idx.WorkingDir+"/"+Idx.OutputDir+"/"+"meta.metadata", os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			return err
		}
	} else {
		metadataFile, err = os.OpenFile(Idx.OutputDir+"/"+"meta.metadata", os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			return err
		}
	}

	defer metadataFile.Close()
	err = metadataFile.Truncate(0)
	if err != nil {
		return err
	}

	_, err = metadataFile.Write(buf.Bytes())

	return err
}

func (Idx *DynamicIndex) LoadMeta() error {
	var metadataFile *os.File
	var err error
	if Idx.WorkingDir != "/" {
		metadataFile, err = os.OpenFile(Idx.WorkingDir+"/"+Idx.OutputDir+"/"+"meta.metadata", os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			return err
		}
	} else {
		metadataFile, err = os.OpenFile(Idx.OutputDir+"/"+"meta.metadata", os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			return err
		}
	}

	defer metadataFile.Close()
	buf := make([]byte, 1024*1024*40)
	metadataFile.Read(buf)
	save := SpimiIndexMetadata{}
	dec := gob.NewDecoder(bytes.NewReader(buf))
	err = dec.Decode(&save)
	if err != nil {
		return err
	}
	Idx.TermIDMap = save.TermIDMap
	Idx.DocWordCount = save.DocWordCount
	Idx.DocsCount = save.DocsCount
	return nil
}

func (Idx *DynamicIndex) GetOutputDir() string {
	return Idx.OutputDir
}

func (Idx *DynamicIndex) GetDocWordCount() map[int]int {
	return Idx.DocWordCount
}

func (Idx *DynamicIndex) GetDocsCount() int {
	return Idx.DocsCount
}

func (Idx *DynamicIndex) GetTermIDMap() pkg.IDMap {
	return Idx.TermIDMap
}

func (Idx *DynamicIndex) BuildVocabulary() {
	Idx.TermIDMap.BuildVocabulary()
}
