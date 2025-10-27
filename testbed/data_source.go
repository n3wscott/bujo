package main

import (
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
)

func loadCollectionsData(useReal bool) ([]*viewmodel.ParsedCollection, error) {
	if useReal {
		return realCollectionsData()
	}
	return sampleCollections(), nil
}

func loadDetailSectionsData(useReal bool) ([]collectiondetail.Section, error) {
	if useReal {
		return realDetailSections()
	}
	return sampleDetailSections(), nil
}
