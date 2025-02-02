package geo

import "math"

type BoundingBox struct {
	min, max []float64 // lat, lon
}

func (bb *BoundingBox) GetMin() []float64 {
	return bb.min
}

func (bb *BoundingBox) GetMax() []float64 {
	return bb.max
}

func NewBoundingBox(lats, lons []float64) BoundingBox {
	min, max := []float64{lats[0], lons[0]}, []float64{lats[0], lons[0]}
	for i := 1; i < len(lats); i++ {
		if lats[i] < min[0] {
			min[0] = lats[i]
		}
		if lats[i] > max[0] {
			max[0] = lats[i]
		}
		if lons[i] < min[1] {
			min[1] = lons[i]
		}
		if lons[i] > max[1] {
			max[1] = lons[i]
		}
	}
	return BoundingBox{
		min: min,
		max: max,
	}
}

func (bb *BoundingBox) Contains(lat, lon float64) bool {
	if lat < bb.min[0] || lat > bb.max[0] {
		return false
	}
	if lon < bb.min[1] || lon > bb.max[1] {
		return false
	}
	return true
}

func (bb *BoundingBox) PointsContains(lats, lons []float64) bool {
	for i := 0; i < len(lats); i++ {
		if !bb.Contains(lats[i], lons[i]) {
			return false
		}
	}
	return true
}

// https://www.movable-type.co.uk/scripts/latlong.html
func MidPoint(lat1, lon1 float64, lat2, lon2 float64) (float64, float64) {
	p1LatRad := degToRad(lat1)
	p2LatRad := degToRad(lat2)

	diffLon := degToRad(lon2 - lon1)

	bx := math.Cos(p2LatRad) * math.Cos(diffLon)
	by := math.Cos(p2LatRad) * math.Sin(diffLon)

	newLon := degToRad(lon1) + math.Atan2(by, math.Cos(p1LatRad)+bx)
	newLat := math.Atan2(math.Sin(p1LatRad)+math.Sin(p2LatRad), math.Sqrt((math.Cos(p1LatRad)+bx)*(math.Cos(p1LatRad)+bx)+by*by))

	return radToDeg(newLat), radToDeg(newLon)
}

func degToRad(d float64) float64 {
	return d * math.Pi / 180.0
}

func radToDeg(r float64) float64 {
	return 180.0 * r / math.Pi
}

// https://www.eecs.umich.edu/courses/eecs380/HANDOUTS/PROJ2/InsidePoly.html
func IsPointInPolygon(pointLat, pointLon float64, polygonLat, polygonLon []float64) bool {
	numVertices := len(polygonLat)
	lon := pointLon
	lat := pointLat
	counter := 0

	p1Lat, p1Lon := polygonLat[0], polygonLon[0]
	var p2Lat, p2Lon float64

	// loop throught each edge
	for i := 1; i <= numVertices; i++ {
		// get next point
		p2Lat, p2Lon = polygonLat[i%numVertices], polygonLon[i%numVertices]

		// check if point is above the minimum lat
		if lat > math.Min(p1Lat, p2Lat) {
			// check if point is below the maximum lat
			if lat <= math.Max(p1Lat, p2Lat) {
				// check if point is to left of the maximum lon
				if lon <= math.Max(p1Lon, p2Lon) {
					if p1Lat != p2Lat {
						lonIntersection := (lat-p1Lat)*(p2Lon-p1Lon)/(p2Lat-p1Lat) + p1Lon

						if p1Lon == p2Lon || lon <= lonIntersection {
							counter++
						}
					}
				}
			}
		}

		p1Lat, p1Lon = p2Lat, p2Lon
	}

	if counter%2 == 0 {
		return false
	} else {
		return true
	}

}

func isLeft(hLat, hLon, tLat, tLon, qLat, qLon float64) float64 {
	return ((tLon - hLon) * (qLat - hLat)) - ((qLon - hLon) * (tLat - hLat))
}

func windingNumber(pLat, pLon float64, polygonLat, polygonLon []float64) (wn int) {

	for i := range polygonLat[:len(polygonLon)-1] {
		if polygonLat[i] <= pLat {
			if polygonLat[i+1] > pLat &&
				isLeft(polygonLat[i], polygonLon[i], polygonLat[i+1], polygonLon[i+1], pLat, pLon) > 0 {
				wn++
			}
		} else if polygonLat[i+1] <= pLat &&
			isLeft(polygonLat[i], polygonLon[i], polygonLat[i+1], polygonLon[i+1], pLat, pLon) < 0 {
			wn--
		}
	}
	return
}

func IsPointInsidePolygonWindingNum(pLat, pLon float64, polygonLat, polygonLon []float64) bool {
	return windingNumber(pLat, pLon, polygonLat, polygonLon) != 0
}
