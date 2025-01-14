package pkg

import (
	"container/heap"
	"fmt"
	"math"
	"sort"

	"github.com/RadhiFadlillah/go-sastrawi"
)

type DynamicIndexer interface {
	GetOutputDir() string
	GetDocWordCount() map[int]int
	GetDocsCount() int
	GetTermIDMap() IDMap
	BuildVocabulary()
}

type SearcherDocStore interface {
	GetDoc(docID int) (Node, error)
}

type Searcher struct {
	Idx DynamicIndexer
	// KV             SearcherKVDB
	MainIndex      *InvertedIndex
	SpellCorrector SpellCorrectorI
	TermIDMap      IDMap
	DocStore       SearcherDocStore
}

func NewSearcher(idx DynamicIndexer, docStore SearcherDocStore, spell SpellCorrectorI) *Searcher {

	return &Searcher{Idx: idx, DocStore: docStore, SpellCorrector: spell}
}

func (se *Searcher) LoadMainIndex() error {
	mainIndex := NewInvertedIndex("merged_index", se.Idx.GetOutputDir())
	err := mainIndex.OpenReader()
	if err != nil {
		return err
	}
	se.MainIndex = mainIndex
	se.Idx.BuildVocabulary()
	se.TermIDMap = se.Idx.GetTermIDMap()
	return nil
}

func (se *Searcher) Close() {
	se.MainIndex.Close()
}

type PostingsResult struct {
	Postings []int
	Err      error
	TermID   int
}

func NewPostingsResult(postings []int, err error, termID int) PostingsResult {
	return PostingsResult{
		Postings: postings,
		Err:      err,
		TermID:   termID,
	}
}

func (p *PostingsResult) GetError() error {
	return p.Err
}

func (p *PostingsResult) GetTermID() int {
	return p.TermID
}

func (p *PostingsResult) GetPostings() []int {
	return p.Postings
}

func (se *Searcher) GetPostingListCon(termID int) PostingsResult {
	postings, err := se.MainIndex.GetPostingList(termID)
	if err != nil {
		return NewPostingsResult([]int{}, err, termID)
	}
	return NewPostingsResult(postings, nil, termID)
}

type DocWithScore struct {
	DocID int
	Score float64
}

// https://nlp.stanford.edu/IR-book/pdf/06vect.pdf (figure 6.14 bagian function COSINESCORE(q))
func (se *Searcher) FreeFormQuery(query string, k int) ([]Node, error) {
	if k == 0 {
		k = 10
	}
	documentScore := make(map[int]float64) // menyimpan skor cosine tf-idf docs \dot tf-idf query

	queryTerms := sastrawi.Tokenize(query)

	queryWordCount := make(map[int]int, len(queryTerms))

	queryTermsID := make([]int, 0, len(queryTerms))

	allPostings := make(map[int][]int, len(queryTerms))

	// {{term1,term1OneEdit}, {term2, term2Edit}, ...}
	allPossibleQueryTerms := make([][]int, len(queryTerms))
	originalQueryTerms := make([]int, 0, len(queryTerms))

	for i, term := range queryTerms {
		tokenizedTerm := stemmer.Stem(term)
		isInVocab := se.TermIDMap.IsInVocabulary(tokenizedTerm)

		originalQueryTerms = append(originalQueryTerms, se.TermIDMap.GetID(tokenizedTerm))

		if !isInVocab {

			correctionOne, err := se.SpellCorrector.GetWordCandidates(tokenizedTerm, 1)
			if err != nil {
				return []Node{}, err
			}
			correctionTwo, err := se.SpellCorrector.GetWordCandidates(tokenizedTerm, 2)
			if err != nil {
				return []Node{}, err
			}
			allPossibleQueryTerms[i] = append(allPossibleQueryTerms[i], correctionOne...)
			allPossibleQueryTerms[i] = append(allPossibleQueryTerms[i], correctionTwo...)
		} else {
			termID := se.TermIDMap.GetID(tokenizedTerm)
			allPossibleQueryTerms[i] = []int{termID}
		}
	}

	allCorrectQueryCandidates := se.SpellCorrector.GetCorrectQueryCandidates(allPossibleQueryTerms)
	correctQuery, err := se.SpellCorrector.GetCorrectSpellingSuggestion(allCorrectQueryCandidates, originalQueryTerms)

	if err != nil {
		return []Node{}, err
	}

	queryTermsID = append(queryTermsID, correctQuery...)

	for _, termID := range queryTermsID {
		postings, err := se.MainIndex.GetPostingList(termID)
		if err != nil {
			return []Node{}, err
		}
		allPostings[termID] = postings
		queryWordCount[termID] += 1
	}

	docsCount := float64(se.Idx.GetDocsCount())
	docNorm := make(map[int]float64)
	queryNorm := 0.0
	for qTermID, postings := range allPostings {
		// iterate semua term di query, hitung tf-idf query dan tf-idf document, accumulate skor cosine di docScore
		// https://web.stanford.edu/~jurafsky/slp3/14.pdf

		termCountInDoc := make(map[int]int)
		for _, docID := range postings {
			termCountInDoc[docID]++ // conunt(t,d)
		}

		tfTermQuery := 1 + math.Log10(float64(queryWordCount[qTermID]))                  //  1 + log(count(t,q))
		idfTermQuery := math.Log10(docsCount) - math.Log10(float64(len(termCountInDoc))) // log(N/df_t)
		tfIDFTermQuery := tfTermQuery * idfTermQuery

		for docID, termCount := range termCountInDoc {
			tf := 1 + math.Log10(float64(termCount)) //  //  1 + log(count(t,d))

			tfIDFTermDoc := tf * idfTermQuery //tfidf docID

			documentScore[docID] += tfIDFTermDoc * tfIDFTermQuery // summation tfidfDoc*tfIDfquery over query terms

			docNorm[docID] += tfIDFTermDoc * tfIDFTermDoc // document Norm
		}

		queryNorm += tfIDFTermQuery * tfIDFTermQuery
	}

	queryNorm = math.Sqrt(queryNorm)

	docWithScores := make([]DocWithScore, 0, len(documentScore))
	// normalize dengan cara dibagi dengan norm vector query & document
	for docID, score := range documentScore {
		documentScore[docID] = score / (queryNorm * math.Sqrt(docNorm[docID]))
		docWithScores = append(docWithScores, DocWithScore{DocID: docID, Score: documentScore[docID]})
	}

	sort.Slice(docWithScores, func(i, j int) bool {
		return docWithScores[i].Score > docWithScores[j].Score
	})

	relevantDocs := make([]Node, 0, k)
	for i := 0; i < k; i++ {
		if i >= len(docWithScores) {
			break
		}

		doc, err := se.DocStore.GetDoc(docWithScores[i].DocID)
		if err != nil {
			return []Node{}, err
		}
		relevantDocs = append(relevantDocs, doc)
	}

	return relevantDocs, nil
}

func (se *Searcher) Autocomplete(query string) ([]Node, error) {

	queryTerms := sastrawi.Tokenize(query)

	// {{term1,term1OneEdit}, {term2, term2Edit}, ...}
	allPossibleQueryTerms := make([][]int, len(queryTerms))
	originalQueryTerms := make([]int, 0, len(queryTerms))

	for i, term := range queryTerms {
		tokenizedTerm := stemmer.Stem(term)
		// isInVocab := se.TermIDMap.IsInVocabulary(tokenizedTerm)

		originalQueryTerms = append(originalQueryTerms, se.TermIDMap.GetID(tokenizedTerm))

		if i == len(queryTerms)-1 {

			matchedWord, err := se.SpellCorrector.GetMatchedWordBasedOnPrefix(tokenizedTerm)
			if err != nil {
				return []Node{}, err
			}

			allPossibleQueryTerms[i] = append(allPossibleQueryTerms[i], matchedWord...)

		} else {
			termID := se.TermIDMap.GetID(tokenizedTerm)
			allPossibleQueryTerms[i] = []int{termID}
		}
	}

	allCorrectQueryCandidates := se.SpellCorrector.GetCorrectQueryCandidates(allPossibleQueryTerms)
	matchedQueries, err := se.SpellCorrector.GetMatchedWordsAutocomplete(allCorrectQueryCandidates, originalQueryTerms)

	if err != nil {
		return []Node{}, err
	}

	relDocIDs := []int{}
	for _, queryTerms := range matchedQueries {

		tokens := make([]int, 0, len(queryTerms)-1)
		for j, termID := range queryTerms {
			tokens = append(tokens, termID)
			if j != len(queryTerms)-1 {
				tokens = append(tokens, -1) // AND
			}
		}

		// shunting Yard

		rpnDeque := NewDeque(shuntingYardRPN(tokens))
		docIDsRes, err := se.processQuery(rpnDeque)
		if err != nil {
			return []Node{}, err
		}
		relDocIDs = append(relDocIDs, docIDsRes...)
	}

	if len(relDocIDs) >= 10 {
		relDocIDs = relDocIDs[:10]
	}

	relevantDocs := make([]Node, 0, len(relDocIDs))

	for i := 0; i < len(relDocIDs); i++ {

		doc, err := se.DocStore.GetDoc(relDocIDs[i])
		if err != nil {
			return []Node{}, err
		}
		relevantDocs = append(relevantDocs, doc)
	}

	return relevantDocs, nil
}

type Deque struct {
	items []int
}

func NewDeque(items []int) Deque {
	return Deque{items}
}

func (d *Deque) GetSize() int {
	return len(d.items)
}

func (d *Deque) PushFront(item int) {
	d.items = append([]int{item}, d.items...)
}

func (d *Deque) PushBack(item int) {
	d.items = append(d.items, item)
}

func (d *Deque) PopFront() (int, bool) {
	if len(d.items) == 0 {
		return 0, false
	}
	frontElement := d.items[0]
	d.items = d.items[1:]
	return frontElement, true
}

func (d *Deque) PopBack() (int, bool) {
	if len(d.items) == 0 {
		return 0, false
	}
	rearElement := d.items[len(d.items)-1]
	d.items = d.items[:len(d.items)-1]
	return rearElement, true
}

func shuntingYardRPN(tokens []int) []int {
	precedence := make(map[int]int)
	precedence[-1] = 2 // AND
	precedence[-2] = 0 // (
	precedence[-3] = 0 // )
	precedence[-4] = 1 // OR
	precedence[-5] = 3 // NOT

	output := make([]int, 0, len(tokens))
	stack := []int{}

	for _, token := range tokens {
		if token == -2 {
			stack = append(stack, -2)
		} else if token == -3 {
			// pop
			n := len(stack) - 1
			operator := stack[n]
			stack = stack[:n]

			for operator != -2 {
				output = append(output, operator)
				// pop
				n = len(stack) - 1
				operator = stack[n]
				stack = stack[:n]
			}
		} else if _, ok := precedence[token]; ok {
			if len(stack) != 0 {
				n := len(stack) - 1
				operator := stack[n]

				for len(stack) != 0 && precedence[token] < precedence[operator] {
					output = append(output, operator)
					n = len(stack) - 1
					stack = stack[:n]
					if len(stack) != 0 {
						n = len(stack) - 1
						operator = stack[n]
					}
				}
			}

			stack = append(stack, token)
		} else {
			// term
			output = append(output, token)
		}
	}

	for len(stack) != 0 {
		n := len(stack) - 1
		token := stack[n]
		stack = stack[:n]
		output = append(output, token)
	}
	return output
}

// processQuery. process query -> return hasil boolean query (AND/OR/NOT) berupa posting lists (docIDs)
func (se *Searcher) processQuery(rpnDeque Deque) ([]int, error) {
	operator := map[int]struct{}{
		-1: struct{}{},
		-5: struct{}{},
		-4: struct{}{},
	}
	postingListStack := []SkipListsReader{}
	for rpnDeque.GetSize() != 0 {
		token, valid := rpnDeque.PopFront()
		if !valid {
			return []int{}, fmt.Errorf("rpn deque size is 0")
		}

		if _, ok := operator[token]; !ok {
			postingList, err := se.MainIndex.GetPostingListSkipList(token)
			if err != nil {
				return []int{}, fmt.Errorf("error when get posting list skip list: %w", err)
			}
			postingListStack = append(postingListStack, NewSkipListsReader(postingList))
		} else {

			if token == -1 {
				// AND
				right := postingListStack[len(postingListStack)-1]
				postingListStack = postingListStack[:len(postingListStack)-1]
				left := postingListStack[len(postingListStack)-1]
				postingListStack = postingListStack[:len(postingListStack)-1]

				postingListIntersection := FastPostingListsIntersection(left, right)

				resultSkipList := NewSkipLists()
				for _, docID := range postingListIntersection {
					resultSkipList.Insert(docID)
				}

				postingListStack = append(postingListStack, NewSkipListsReader(resultSkipList.Serialize()))
			} else if token == -4 {
				// OR
				// NOT IMPLEMENTED YET
			} else {
				// NOT
				// NOT IMPLEMENTED YET
			}
		}
	}

	docIDsResult, err := postingListStack[len(postingListStack)-1].GetAllItems()
	if err != nil {
		return []int{}, err
	}
	return docIDsResult, nil
}

func (se *Searcher) FreeFormQueryWithoutDocs(query string, k int) ([]int, error) {
	if k == 0 {
		k = 10
	}
	documentScore := make(map[int]float64) // menyimpan skor cosine tf-idf docs \dot tf-idf query

	docsPQ := NewMaxPriorityQueue[int, float64]()
	heap.Init(docsPQ)

	queryTerms := sastrawi.Tokenize(query)

	queryWordCount := make(map[int]int, len(queryTerms))

	queryTermsID := make([]int, 0, len(queryTerms))

	allPostings := make(map[int][]int, len(queryTerms))

	// {{term1,term1OneEdit}, {term2, term2Edit}, ...}
	allPossibleQueryTerms := make([][]int, len(queryTerms))
	originalQueryTerms := make([]int, 0, len(queryTerms))

	for i, term := range queryTerms {
		tokenizedTerm := stemmer.Stem(term)
		isInVocab := se.TermIDMap.IsInVocabulary(tokenizedTerm)

		originalQueryTerms = append(originalQueryTerms, se.TermIDMap.GetID(tokenizedTerm))

		if !isInVocab {

			correctionOne, err := se.SpellCorrector.GetWordCandidates(tokenizedTerm, 1)
			if err != nil {
				return []int{}, err
			}
			correctionTwo, err := se.SpellCorrector.GetWordCandidates(tokenizedTerm, 2)
			if err != nil {
				return []int{}, err
			}
			allPossibleQueryTerms[i] = append(allPossibleQueryTerms[i], correctionOne...)
			allPossibleQueryTerms[i] = append(allPossibleQueryTerms[i], correctionTwo...)
		} else {
			termID := se.TermIDMap.GetID(tokenizedTerm)
			allPossibleQueryTerms[i] = []int{termID}
		}
	}

	allCorrectQueryCandidates := se.SpellCorrector.GetCorrectQueryCandidates(allPossibleQueryTerms)
	correctQuery, err := se.SpellCorrector.GetCorrectSpellingSuggestion(allCorrectQueryCandidates, originalQueryTerms)

	if err != nil {
		return []int{}, err
	}

	queryTermsID = append(queryTermsID, correctQuery...)

	for _, termID := range queryTermsID {
		postings, err := se.MainIndex.GetPostingList(termID)
		if err != nil {
			return []int{}, err
		}
		allPostings[termID] = postings
		queryWordCount[termID] += 1
	}

	docsCount := float64(se.Idx.GetDocsCount())
	docNorm := make(map[int]float64)
	queryNorm := 0.0
	for qTermID, postings := range allPostings {
		// iterate semua term di query, hitung tf-idf query dan tf-idf document, accumulate skor cosine di docScore
		// https://web.stanford.edu/~jurafsky/slp3/14.pdf

		termCountInDoc := make(map[int]int)
		for _, docID := range postings {
			termCountInDoc[docID]++ // conunt(t,d)
		}

		tfTermQuery := 1 + math.Log10(float64(queryWordCount[qTermID]))                  //  1 + log(count(t,q))
		idfTermQuery := math.Log10(docsCount) - math.Log10(float64(len(termCountInDoc))) // log(N/df_t)
		tfIDFTermQuery := tfTermQuery * idfTermQuery

		for docID, termCount := range termCountInDoc {
			tf := 1 + math.Log10(float64(termCount)) //  //  1 + log(count(t,d))

			tfIDFTermDoc := tf * idfTermQuery //tfidf docID

			documentScore[docID] += tfIDFTermDoc * tfIDFTermQuery // summation tfidfDoc*tfIDfquery over query terms

			docNorm[docID] += tfIDFTermDoc * tfIDFTermDoc // document Norm
		}

		queryNorm += tfIDFTermQuery * tfIDFTermQuery
	}

	queryNorm = math.Sqrt(queryNorm)

	docWithScores := make([]DocWithScore, 0, len(documentScore))
	// normalize dengan cara dibagi dengan norm vector query & document
	for docID, score := range documentScore {
		documentScore[docID] = score / (queryNorm * math.Sqrt(docNorm[docID]))
		docWithScores = append(docWithScores, DocWithScore{DocID: docID, Score: documentScore[docID]})
	}

	sort.Slice(docWithScores, func(i, j int) bool {
		return docWithScores[i].Score > docWithScores[j].Score
	})

	relevantDocs := make([]int, 0, k)
	for i := 0; i < k; i++ {
		if i >= len(docWithScores) {
			break
		}

		currRelDocID := docWithScores[i].DocID

		relevantDocs = append(relevantDocs, currRelDocID)
	}

	return relevantDocs, nil
}
