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

var treeType = map[string]string{
	"40":  "child tree (directory)",
	"100": "child blob (file)",
	"120": "symlink",
	"160": "submodule",
}

type object struct {
	Type string
	Size int
}

type TreeObject struct {
	object
	Body []TreeFile
}

type TreeFile struct {
	Name string
	Type string
	Mode string
	Hash string
}

type CommitObject struct {
	object
	Body *Commit
}

type Commit struct {
	Tree      *TreeObject
	Parent    []string
	Author    *Author
	Committer *Committer
}

type person struct {
	Username  string
	Email     string
	Timestamp string
}

type BlobObject struct {
	object
	Body []byte
}

type Tag struct {
	Object  string
	Type    string
	Name    string
	Tagger  string
	Comment string
}

type TagObject struct {
	object
	Body *Tag
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
		case "parent":
			commit.Parent = append(commit.Parent, sp[1])
		case "author", "committer":
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

func ParseTree(obj *TreeObject, buf *bytes.Buffer) error {
	for _, s := range strings.Split(buf.String(), ":") {
		var file TreeFile
		sp := strings.Split(s, " ")
		file.Type = treeType[sp[0][0:len(sp[0])-3]]
		file.Mode = sp[0][len(sp[0])-3:]

		for i, b := range []byte(sp[1]) {
			if b == 0x00 {
				file.Name = string(sp[1][:i])
				file.Hash = fmt.Sprintf("%x", sp[1][i+1:])
			}
		}
		obj.Body = append(obj.Body, file)
	}
	return nil
}

func ParseCommit(obj *CommitObject, buf *bytes.Buffer) error {
	// find tree hash
	bi, err := buf.ReadBytes('\n')
	if err != nil {
		return err
	}
	hash := strings.Split(string(bi[:len(bi)-1]), " ")[1]
	b, err := openObject(hash)
	if err != nil {
		return err
	}

	s := bufio.NewScanner(buf)
	commit, err := parseCommit(s)
	if err != nil {
		return err
	}
	obj.Body = commit

	bobj, err := Parse(b)
	if err != nil {
		return err
	}
	obj.Body.Tree = bobj.(*TreeObject)

	return nil
}

func ParseBlob(obj *BlobObject, buf *bytes.Buffer) {
	obj.Body = buf.Bytes()
}

type headerReader struct {
	b   *bytes.Buffer
	err error
}

func (r *headerReader) readString(b byte) string {
	if r.err != nil {
		return ""
	}
	var s string
	s, r.err = r.b.ReadString(b)
	if r.err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(s, " ", 2)[1])
}

func ParseTag(obj *TagObject, buf *bytes.Buffer) error {
	tag := &Tag{}
	r := headerReader{b: buf}
	tag.Object = r.readString('\n')
	tag.Type = r.readString('\n')
	tag.Name = r.readString('\n')
	tag.Tagger = r.readString('\n')
	buf.ReadString('\n') // skip empty line
	var err error
	tag.Comment, err = buf.ReadString('\n')
	if err != nil {
		return err
	}
	tag.Comment = strings.TrimSpace(tag.Comment)
	obj.Body = tag
	return r.err
}

func Parse(buf *bytes.Buffer) (interface{}, error) {
	var obj object
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

	switch obj.Type {
	case "commit":
		obj := &CommitObject{object: obj}
		err = ParseCommit(obj, buf)
		return obj, err
	case "tree":
		obj := &TreeObject{object: obj}
		err = ParseTree(obj, buf)
		return obj, err
	case "blob":
		obj := &BlobObject{object: obj}
		ParseBlob(obj, buf)
		return obj, nil
	case "tag":
		obj := &TagObject{object: obj}
		ParseTag(obj, buf)
		return obj, nil
	default:
		return nil, fmt.Errorf("unknown type: %s", obj.Type)
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func openObject(name string) (*bytes.Buffer, error) {
	p := filepath.Join(".git", "objects", name[:2], name[2:])
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := zlib.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, r)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func main() {
	if len(os.Args) != 2 {
		panic("one argument required")
	}

	buf, err := openObject(os.Args[1])
	if err != nil {
		panic(err)
	}

	obj, err := Parse(buf)
	if err != nil {
		panic(err)
	}

	pp.Println(obj)
}
