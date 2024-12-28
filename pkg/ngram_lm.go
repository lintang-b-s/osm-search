package pkg

import (
	"bytes"
	"encoding/gob"
	"os"
)

const (
	UNKNOWN_TOKEN = "<UNK>"
	START_TOKEN   = "<s>"
	END_TOKEN     = "</s>"
)

type NGramLanguageModel struct {
	Vocabulary []int
	WordCounts map[int]int
	TermIDMap  IDMap
	Data       NGramData
}

type NGramData struct {
	OneGramCount   map[int]int
	TwoGramCount   map[[2]int]int
	ThreeGramCount map[[3]int]int
	FourGramCount  map[[4]int]int
	TotalWordFreq  int
}

func NewNGramLanguageModel() *NGramLanguageModel {
	return &NGramLanguageModel{
		Vocabulary: make([]int, 0),
		WordCounts: make(map[int]int),
		TermIDMap:  NewIDMap(),
	}
}

func (lm *NGramLanguageModel) AddWord(word int) {
	lm.WordCounts[word]++
	if _, ok := lm.WordCounts[word]; !ok {
		lm.Vocabulary = append(lm.Vocabulary, word)
	}
}

// CountWords. menghitung frekuensi setiap kata dalam corpus
func (lm *NGramLanguageModel) CountWords(tokenizedDocs [][]string) {
	for _, doc := range tokenizedDocs {

		for _, word := range doc {
			wordID := lm.TermIDMap.GetID(word)
			lm.AddWord(wordID)
		}
	}
}

/*
GetWordsWithNPlusFreq. return kata-kata yang memiliki frekuensi lebih dari countThresold. kata yang kurang dari thresold jadi <UNK>
*/
func (lm *NGramLanguageModel) GetWordsWithNPlusFreq(tokenizedDocs [][]string, countThresold int) []int {
	lm.CountWords(tokenizedDocs)
	closedWords := make([]int, 0)
	for word, count := range lm.WordCounts {
		if count >= countThresold {
			closedWords = append(closedWords, word)
		}
	}
	return closedWords
}

// ReplaceOOVWordsWithUNK. mengganti kata-kata yang frequensinya < 2 dengan <UNK>
func (lm *NGramLanguageModel) ReplaceOOVWordsWithUNK(tokenizedDocs [][]string, vocabulary []int) [][]int {
	replacedTokenizedDocs := [][]int{}

	unknownTokenID := lm.TermIDMap.GetID(UNKNOWN_TOKEN)
	vocabSet := make(map[int]bool)
	for _, word := range vocabulary {
		vocabSet[word] = true
	}

	for _, doc := range tokenizedDocs {
		replacedDoc := []int{}
		for _, token := range doc {
			tokenID := lm.TermIDMap.GetID(token)
			if _, ok := vocabSet[tokenID]; ok {
				replacedDoc = append(replacedDoc, tokenID)
			} else {
				replacedDoc = append(replacedDoc, unknownTokenID)
			}
		}
		replacedTokenizedDocs = append(replacedTokenizedDocs, replacedDoc)
	}
	return replacedTokenizedDocs
}

func (lm *NGramLanguageModel) PreProcessData(tokenizedDocs [][]string, countThresold int) [][]int {
	lm.CountWords(tokenizedDocs)
	vocabulary := lm.GetWordsWithNPlusFreq(tokenizedDocs, countThresold)
	replacedTokenizedDocs := lm.ReplaceOOVWordsWithUNK(tokenizedDocs, vocabulary)
	return replacedTokenizedDocs
}

func (lm *NGramLanguageModel) CountOnegram(data [][]int) {

	var nGrams = make(map[int]int)

	for _, doc := range data {

		doc = lm.AddStartEndToken(doc, 1)

		m := len(doc)
		for i := 0; i < m; i++ {
			nGram := doc[i]

			if _, ok := nGrams[nGram]; !ok {
				nGrams[nGram] = 1
			} else {
				nGrams[nGram]++
			}

			lm.Data.TotalWordFreq++
		}
	}

	lm.Data.OneGramCount = nGrams
}

func (lm *NGramLanguageModel) CountTwogram(data [][]int) {

	var nGrams = make(map[[2]int]int)

	for _, doc := range data {

		doc = lm.AddStartEndToken(doc, 2)

		m := len(doc) - 2 + 1
		for i := 0; i < m; i++ {
			var nGram [2]int

			copy(nGram[:], doc[i:i+2])

			if _, ok := nGrams[nGram]; !ok {
				nGrams[nGram] = 1
			} else {
				nGrams[nGram]++
			}
		}
	}

	lm.Data.TwoGramCount = nGrams
}

func (lm *NGramLanguageModel) CountThreegram(data [][]int) {

	var nGrams = make(map[[3]int]int)

	for _, doc := range data {

		doc = lm.AddStartEndToken(doc, 3)

		m := len(doc) - 3 + 1
		for i := 0; i < m; i++ {
			var nGram [3]int

			copy(nGram[:], doc[i:i+3])

			if _, ok := nGrams[nGram]; !ok {
				nGrams[nGram] = 1
			} else {
				nGrams[nGram]++
			}
		}
	}

	lm.Data.ThreeGramCount = nGrams
}

func (lm *NGramLanguageModel) CountFourgram(data [][]int) {

	var nGrams = make(map[[4]int]int)

	for _, doc := range data {

		doc = lm.AddStartEndToken(doc, 4)

		m := len(doc) - 4 + 1
		for i := 0; i < m; i++ {
			var nGram [4]int

			copy(nGram[:], doc[i:i+4])

			if _, ok := nGrams[nGram]; !ok {
				nGrams[nGram] = 1
			} else {
				nGrams[nGram]++
			}
		}
	}

	lm.Data.FourGramCount = nGrams
}

// EstimateProbability. menghitung probabilitas nextWord berdasarkan previous tokens.
func (lm *NGramLanguageModel) EstimateProbability(nextWord int, previousNGram []int, n int) float64 {
	switch n {
	case 1:
		var ngramCount int
		if count, ok := lm.Data.OneGramCount[nextWord]; ok {
			ngramCount = count
		} else {
			ngramCount = 0
		}

		denominator := lm.Data.TotalWordFreq
		numerator := ngramCount
		probability := float64(numerator) / float64(denominator)
		return probability

	case 2:
		prevNGram := [1]int{previousNGram[0]}
		var prevNgramCount int
		if count, ok := lm.Data.OneGramCount[nextWord]; ok {
			prevNgramCount = count
		} else {
			prevNgramCount = 0
		}
		denominator := prevNgramCount

		nGram := [2]int{prevNGram[0], nextWord}

		var nGramCount int
		if count, ok := lm.Data.TwoGramCount[nGram]; ok {
			nGramCount = count
		} else {
			nGramCount = 0
		}

		numerator := nGramCount

		probability := float64(numerator) / float64(denominator)
		return probability
	case 3:
		prevNGram := [2]int{previousNGram[0], previousNGram[1]}
		var prevNgramCount int
		if count, ok := lm.Data.TwoGramCount[prevNGram]; ok {
			prevNgramCount = count
		} else {
			prevNgramCount = 0
		}
		denominator := prevNgramCount

		nGram := [3]int{prevNGram[0], prevNGram[1], nextWord}

		var nGramCount int
		if count, ok := lm.Data.ThreeGramCount[nGram]; ok {
			nGramCount = count
		} else {
			nGramCount = 0
		}

		numerator := nGramCount

		probability := float64(numerator) / float64(denominator)
		return probability
	case 4:
		prevNGram := [3]int{previousNGram[0], previousNGram[1], previousNGram[2]}
		var prevNgramCount int
		if count, ok := lm.Data.ThreeGramCount[prevNGram]; ok {
			prevNgramCount = count
		} else {
			prevNgramCount = 0
		}
		denominator := prevNgramCount

		nGram := [4]int{prevNGram[0], prevNGram[1], prevNGram[2], nextWord}

		var nGramCount int
		if count, ok := lm.Data.FourGramCount[nGram]; ok {
			nGramCount = count
		} else {
			nGramCount = 0
		}

		numerator := nGramCount

		probability := float64(numerator) / float64(denominator)
		return probability
	}
	return 0
}

// EstimateProbabilities. menghitung probabilitas nextWordCandidates berdasarkan previous tokens.
func (lm *NGramLanguageModel) EstimateWordCandidatesProbabilities(nextWordCandidates []int, prevNgrams []int, n int) map[int]float64 {
	var nextWordProbabilities = make(map[int]float64)

	for _, nextWord := range nextWordCandidates {
		probability := lm.EstimateProbability(nextWord, prevNgrams, n)
		nextWordProbabilities[nextWord] = probability
	}
	return nextWordProbabilities
}

// MakeCountMatrix. menghitung frekuensi n-gram dari data
func (lm *NGramLanguageModel) MakeCountMatrix(data [][]int) {
	lm.CountOnegram(data)
	lm.CountTwogram(data)
	lm.CountThreegram(data)
	lm.CountFourgram(data)
}

// AddStartEndToken. menambahkan token <s> sebanyak n dan </s> pada awal dan akhir dokumen
func (lm *NGramLanguageModel) AddStartEndToken(doc []int, n int) []int {
	startToken := []int{}
	startTokenID := lm.TermIDMap.GetID(START_TOKEN)
	endTokenID := lm.TermIDMap.GetID(END_TOKEN)

	for i := 0; i < n; i++ {
		startToken = append(startToken, startTokenID)
	}
	doc = append(startToken, doc...)
	doc = append(doc, endTokenID)
	return doc
}

func (lm *NGramLanguageModel) SaveNGramData() error {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(lm.Data)
	if err != nil {
		return err
	}

	ngramFile, err := os.OpenFile("ngram.index", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer ngramFile.Close()
	err = ngramFile.Truncate(0)
	if err != nil {
		return err
	}

	_, err = ngramFile.Write(buf.Bytes())

	return err
}

func (lm *NGramLanguageModel) LoadNGramData() error {
	ngramFile, err := os.Open("ngram.index")
	if err != nil {
		return err
	}
	defer ngramFile.Close()

	dec := gob.NewDecoder(ngramFile)
	err = dec.Decode(&lm.Data)
	if err != nil {
		return err
	}

	return nil
}