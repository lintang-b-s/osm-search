package pkg

// Node ya tempat/jalan yang ada di osm. yang di index = nama + alamat + building
type Node struct {
	ID      int       `json:"id"` // 32 bit/ 4 byte
	Name    [64]byte  `json:"name"` // dari tag name osm // 8bit * 64 = 64 character
	Lat     float64   `json:"lat"`  // dari center  point polygon  osm way
	Lon     float64   `json:"lon"`
	Address [128]byte `json:"address"` // dari tag addr:city/addr:street/addr:place/dll osm, digabungin pakai koma // 128 karakter
	City    [32]byte  `json:"city"`    // dari tag addr:city osm
	Tipe    [64]byte  `json:"type"`    // dari value tag amenity / building osm atau historic kalau node
} // buffer size = 4 + 64 + 8 + 8 + 128 + 64 +32 = 308 byte

func NewNode(id int, name string, lat float64, lon float64, address string, tipe string, city string) Node {
	var nameB [64]byte
	copy(nameB[:], name)
	var addressB [128]byte
	copy(addressB[:], address)
	var tipeB [64]byte
	copy(tipeB[:], tipe)
	var cityB [32]byte
	copy(cityB[:], city)
	return Node{
		ID:      id,
		Name:    nameB,
		Lat:     lat,
		Lon:     lon,
		Address: addressB,
		Tipe:    tipeB,
		City:    cityB,
	}
}
