package pkg

import (
	"context"
	"io"
	"os"

	"github.com/k0kubun/go-ansi"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
	"github.com/schollz/progressbar/v3"
)

type nodeMapContainer struct {
	nodeMap map[int]*osm.Node
}

var ValidSearchTags = map[string]bool{
	"amenity":          true,
	"building":         true,
	"sport":            true,
	"tourism":          true,
	"leisure":          true,
	"boundary":         true,
	"landuse":          true,
	"craft":            true,
	"aeroway":          true,
	"historic":         true,
	"residential":      true,
	"public_transport": true,
	"railway":          true,
	"shop":             true,
	"junction":         true,
	"route":            true,
	"ferry":            true,
	"highway":          true,
	"motorcar":         true,
	"motor_vehicle":    true,
	"access":           true,
	"industrial":       true,
}

var ValidNodeSearchTag = map[string]bool{
	"historic": true,
	"name":     true,
}

type OSMWay struct {
	NodeIDs []int
	TagMap  map[int]int
}

func NewOSMWay(nodeIDs []int, tagMap map[int]int) OSMWay {
	return OSMWay{
		NodeIDs: nodeIDs,
		TagMap:  tagMap,
	}
}

type OSMNode struct {
	Lat    float64
	Lon    float64
	TagMap map[int]int
}

func NewOSMNode(lat float64, lon float64, tagMap map[int]int) OSMNode {
	return OSMNode{
		Lat:    lat,
		Lon:    lon,
		TagMap: tagMap,
	}
}

func ParseOSM(mapfile string) ([]OSMWay, []OSMNode, nodeMapContainer, IDMap, error) {
	var TagIDMap IDMap = NewIDMap()

	f, err := os.Open(mapfile)

	if err != nil {
		return []OSMWay{}, []OSMNode{}, nodeMapContainer{}, IDMap{}, err
	}

	defer f.Close()

	scanner := osmpbf.New(context.Background(), f, 3)
	defer scanner.Close()

	count := 0

	ctr := nodeMapContainer{
		nodeMap: make(map[int]*osm.Node),
	}

	ways := []OSMWay{}
	bar := progressbar.NewOptions(3,
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetDescription("[cyan][1/2]Parsing osm objects..."),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
	bar.Add(1)
	wayNodesMap := make(map[osm.NodeID]bool)
	for scanner.Scan() {
		o := scanner.Object()
		tipe := o.ObjectID().Type()

		if tipe != "way" {
			continue
		}
		tag := o.(*osm.Way).TagMap()

		if !checkIsWayAlowed(tag) {
			continue
		}
		name, _, _, _ := GetNameAddressTypeFromOSMWay(tag)
		if name == "" {
			continue
		}

		myTag := make(map[int]int)
		for k, v := range tag {
			myTag[TagIDMap.GetID(k)] = TagIDMap.GetID(v)
		}

		if tipe == osm.TypeWay {
			nodeIDs := []int{}
			for _, node := range o.(*osm.Way).Nodes {
				wayNodesMap[node.ID] = true
				nodeIDs = append(nodeIDs, int(node.ID))
			}
			way := NewOSMWay(nodeIDs, myTag)
			ways = append(ways, way)
		}
		count++
	}

	bar.Add(1)
	f.Seek(0, io.SeekStart)
	if err != nil {
		return []OSMWay{}, []OSMNode{}, nodeMapContainer{}, IDMap{}, err
	}

	scanner = osmpbf.New(context.Background(), f, 3)
	defer scanner.Close()

	onlyOsmNodes := []OSMNode{}
	for scanner.Scan() {
		o := scanner.Object()
		if o.ObjectID().Type() == osm.TypeNode {
			node := o.(*osm.Node)
			if _, ok := wayNodesMap[node.ID]; ok {
				ctr.nodeMap[int(o.(*osm.Node).ID)] = o.(*osm.Node)
			}
			name, _, _, _ := GetNameAddressTypeFromOSNode(node.TagMap())
			if name == "" {
				continue
			}
			if checkIsNodeAlowed(node.TagMap()) {
				lat := node.Lat
				lon := node.Lon
				tag := node.TagMap()

				myTag := make(map[int]int)
				for k, v := range tag {
					myTag[TagIDMap.GetID(k)] = TagIDMap.GetID(v)
				}
				onlyOsmNodes = append(onlyOsmNodes, NewOSMNode(lat, lon, myTag))
			}
		}
	}
	bar.Add(1)

	scanErr := scanner.Err()
	if scanErr != nil {
		return []OSMWay{}, []OSMNode{}, nodeMapContainer{}, IDMap{}, err
	}

	return ways, onlyOsmNodes, ctr, TagIDMap, nil
}

func GetNameAddressTypeFromOSMWay(tag map[string]string) (string, string, string, string) {
	name := tag["name"]
	address := ""
	fullAdress, ok := tag["addr:full"]
	if ok {
		address += fullAdress + ", "
	}
	street, ok := tag["addr:street"]
	if ok {
		address += street + ", "
	}
	place, ok := tag["addr:place"]
	if ok {
		address += place + ", "
	}
	city := ""
	city, ok = tag["addr:city"]
	if ok {
		address += city + ", "
	}
	tipe := GetOSMObjectType(tag)
	return name, address, tipe, city
}

func GetNameAddressTypeFromOSNode(tag map[string]string) (string, string, string, string) {
	name := tag["name"]
	address := ""
	fullAdress, ok := tag["addr:full"]
	if ok {
		address += fullAdress + ", "
	}
	street, ok := tag["addr:street"]
	if ok {
		address += street + ", "
	}
	place, ok := tag["addr:place"]
	if ok {
		address += place + ", "
	}
	city := ""
	city, ok = tag["addr:city"]
	if ok {
		address += city + ", "
	}
	tipe := GetOSMObjectType(tag)
	return name, address, tipe, city
}

func GetOSMObjectType(tag map[string]string) string {
	tipe, ok := tag["amenity"]
	if ok {
		return tipe
	}
	// building tidak include (karena cuma yes/no)
	tipe, ok = tag["historic"]
	if ok {
		return tipe
	}
	tipe, ok = tag["sport"]
	if ok {
		return tipe
	}
	tipe, ok = tag["tourism"]
	if ok {
		return tipe
	}
	tipe, ok = tag["leisure"]
	if ok {
		return tipe
	}
	tipe, ok = tag["landuse"]
	if ok {
		return tipe
	}
	tipe, ok = tag["craft"]
	if ok {
		return tipe
	}
	tipe, ok = tag["aeroway"]
	if ok {
		return tipe
	}
	tipe, ok = tag["residential"]
	if ok {
		return tipe
	}
	tipe, ok = tag["public_transport"]
	if ok {
		return tipe
	}
	tipe, ok = tag["industrial"]
	if ok {
		return tipe
	}
	tipe, ok = tag["shop"]
	if ok {
		return tipe
	}
	return ""
}

func checkIsWayAlowed(tag map[string]string) bool {
	for k, _ := range tag {

		if ValidSearchTags[k] {
			return true
		}
	}
	return false
}

func checkIsNodeAlowed(tag map[string]string) bool {
	for k, _ := range tag {
		if ValidNodeSearchTag[k] {
			return true
		}
	}
	return false
}
