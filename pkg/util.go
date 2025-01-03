package pkg

import "sort"

type IDMap struct {
	StrToID    map[string]int
	IDToStr    map[int]string
	Vocabulary map[string]bool
}

func NewIDMap() IDMap {
	return IDMap{
		StrToID: make(map[string]int),
		IDToStr: make(map[int]string),
	}
}

func (idMap *IDMap) GetID(str string) int {
	if id, ok := idMap.StrToID[str]; ok {
		return id
	}
	id := len(idMap.StrToID)
	idMap.StrToID[str] = id
	idMap.IDToStr[id] = str
	return id
}

func (idMap *IDMap) GetStr(id int) string {
	if str, ok := idMap.IDToStr[id]; ok {
		return str
	}
	return ""
}

func (idMap *IDMap) GetSortedTerms() []string {
	sortedTerms := make([]string, len(idMap.StrToID))
	for term, id := range idMap.StrToID {
		sortedTerms[id] = term
	}
	sort.Strings(sortedTerms)
	return sortedTerms
}

func (idMap *IDMap) BuildVocabulary() {
	idMap.Vocabulary = make(map[string]bool)
	for id := range idMap.StrToID {
		idMap.Vocabulary[id] = true
	}
}

func (idMap *IDMap) GetVocabulary() map[string]bool {
	return idMap.Vocabulary
}

func (idMap *IDMap) IsInVocabulary(term string) bool {
	_, ok := idMap.Vocabulary[term]
	return ok
}

func BinarySearch[T any](arr []T, target T, compare func(a, b T) int) int {
	left := 0
	right := len(arr)
	for left < right {
		mid := left + (right-left)/2
		if compare(arr[mid], target) >= 0 {
			right = mid
		} else {
			left = mid + 1
		}
	}
	return left
}
