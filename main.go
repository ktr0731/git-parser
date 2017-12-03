package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/k0kubun/pp"
)

const (
	typeMaxLen = 6 // "commit" in ["commit", "tree", "blob", "tag"]
)

type Object struct {
	Type string
	Size int
	Body *Commit
}

type Commit struct {
	Tree      string
	Parent    []string
	Author    *Author
	Committer *Committer
}

type person struct {
	Username  string
	Email     string
	Timestamp string
}

type Author person
type Committer person

func parseType(buf *bytes.Buffer) (int, []byte, error) {
	return bufio.ScanWords(buf.Bytes(), false)
}

func parseSize(buf *bytes.Buffer) ([]byte, error) {
	b, err := buf.ReadBytes(0x00)
	if err != nil {
		return nil, err
	}
	return b[:len(b)-1], nil
}

func parseCommit(s *bufio.Scanner) (*Commit, error) {
	var commit Commit
	for s.Scan() {
		line := s.Text()
		sp := strings.Split(line, " ")
		switch sp[0] {
		case "tree":
			commit.Tree = sp[1]
		case "parent":
			commit.Parent = append(commit.Parent, sp[1])
		case "author":
		case "committer":
			username, email, timestamp := sp[1], sp[2], strings.Join(sp[3:], " ")
			if sp[0] == "author" {
				commit.Author = &Author{
					Username:  username,
					Email:     email,
					Timestamp: timestamp,
				}
			} else {
				commit.Committer = &Committer{
					Username:  username,
					Email:     email,
					Timestamp: timestamp,
				}
			}
		}
	}
	return &commit, nil
}

func Parse(buf *bytes.Buffer) (*Object, error) {
	var obj Object
	n, b, err := parseType(buf)
	buf.Next(n)
	if err != nil {
		return nil, err
	}
	obj.Type = string(b)

	b, err = parseSize(buf)
	if err != nil {
		return nil, err
	}
	obj.Size, err = strconv.Atoi(string(b))
	if err != nil {
		return nil, err
	}

	s := bufio.NewScanner(buf)
	s.Scan() // skip header
	commit, err := parseCommit(s)
	if err != nil {
		return nil, err
	}
	obj.Body = commit

	return &obj, nil
}

func main() {
	if len(os.Args) != 2 {
		panic("one argument required")
	}

	p := filepath.Join(".git", "objects", os.Args[1][:2], os.Args[1][2:])
	f, err := os.Open(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	r, err := zlib.NewReader(f)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, r)
	if err != nil {
		panic(err)
	}

	fmt.Println(buf.String())

	obj, err := Parse(buf)
	if err != nil {
		panic(err)
	}

	pp.Println(obj)
}
