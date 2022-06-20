package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

var emptyChange object.ChangeEntry

func name(c *object.Change) string {
	if c.From != emptyChange {
		return c.From.Name
	}
	return c.To.Name
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if e == a {
			return true
		}
	}
	return false
}

func parseCommitLog(cIter object.CommitIter, depth int, extensions []string) (ParseResult, error) {
	files := make(ParseResult, 0, 0)
	count := 0
	err := cIter.ForEach(func(c *object.Commit) error {
		// ignore marge commit
		if len(c.ParentHashes) > 1 {
			dlog.Println("ignore marge commit")
			return nil
		}
		count++
		if count > depth && depth > 0 {
			dlog.Printf("over depth count=%d, depth=%d", count, depth)
			return nil
		}

		ilog.Printf("commit=%d hash:%40s author:%-30s\r", count, c.Hash.String(), c.Author.Name)

		fromTree, err := c.Tree()
		checkIfError(err)

		toTree := &object.Tree{}
		if c.NumParents() != 0 {
			firstParent, err := c.Parents().Next()
			if err != nil {
				return nil
			}

			toTree, err = firstParent.Tree()
			if err != nil {
				return nil
			}
		}

		// very slow...
		diff, err := toTree.Diff(fromTree)
		checkIfError(err)

		for _, v := range diff {
			dlog.Println(v)

			path := FilePath(name(v))
			if !match_extension(extensions, name(v)) {
				continue
			}
			action, _ := v.Action()
			var createBy string = ""
			if action == merkletrie.Insert {
				createBy = c.Author.Name
			}

			if index, val := exists(path, files); val != nil {
				dlog.Printf("exist %s", path)
				if !contains(val.Authors, c.Author.Name) {
					val.Authors = append(val.Authors, c.Author.Name)
				}
				val.CommitHash = append(val.CommitHash, c.Hash.String())
				if createBy != "" {
					val.CreateBy = createBy
				}
				files[index] = *val
			} else {
				dlog.Printf("new file %s", path)
				files = append(files, CommitFile{
					Path:       path,
					Authors:    []string{c.Author.Name},
					CommitHash: []string{c.Hash.String()},
					CreateBy:   createBy,
				})
			}
		}
		return nil
	})
	dlog.Printf("\r\ntotal commit=%d\n", count)

	return files, err
}

func exists(path FilePath, files ParseResult) (int, *CommitFile) {
	for i, v := range files {
		if path == v.Path {
			return i, &v
		}
	}
	return -1, nil
}

func parse(config Config) ParseResult {
	dlog.Println("parse")
	path := config.Path
	outputFile := config.OutputFile
	depth := config.Depth
	extensions := parse_extensions(config.Extensions)

	r, err := git.PlainOpen(path)
	checkIfError(err)

	head, err := r.Head()
	checkIfError(err)

	cIter, err := r.Log(&git.LogOptions{From: head.Hash(), Order: git.LogOrderCommitterTime})
	checkIfError(err)

	files, err := parseCommitLog(cIter, depth, extensions)
	checkIfError(err)

	j, err := json.Marshal(files)
	checkIfError(err)

	var buf bytes.Buffer
	json.Indent(&buf, j, "", "  ")

	if outputFile != "" {
		err := ioutil.WriteFile(outputFile, buf.Bytes(), 0666)
		checkIfError(err)
	}

	return files
}

func open_result(config Config) ParseResult {
	dlog.Println("open_result")
	raw, err := ioutil.ReadFile(config.OutputFile)
	checkIfError(err)

	var parseResult ParseResult

	json.Unmarshal(raw, &parseResult)

	return parseResult
}

func parse_extensions(extensions string) []string {
	extension_slice := strings.Split(extensions, ",")
	if extension_slice[0] == "" {
		extension_slice = []string{}
	}
	return extension_slice
}

func match_extension(extensions []string, path string) bool {
	if len(extensions) == 0 {
		return true
	}
	extensionsRegex := strings.Join(extensions, "|")
	match, _ := regexp.MatchString(`^.*\.(`+extensionsRegex+`+)$`, path)
	return match
}
