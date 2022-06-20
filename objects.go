package main

// file path
type FilePath string
type ParseResult []CommitFile

type CommitFile struct {
	// file path
	Path FilePath

	// Author list
	Authors []string

	CommitHash []string

	CreateBy string
}
