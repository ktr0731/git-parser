package main

import (
	"bufio"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/k0kubun/pp"
)

const (
	typeMaxLen = 6 // "commit" in ["commit", "tree", "blob", "tag"]
)

type Object struct {
	Type string
	Size int
	Body []byte
}

func parseType(r io.Reader) ([]byte, error) {
	buf := make([]byte, typeMaxLen)
	_, err := r.Read(buf)
	if err != nil {
		return nil, err
	}
	for i := range buf {
		if buf[i] == ' ' {
			return buf[:i], nil
		}
	}

	return buf, nil
}

func parseSize(r io.Reader) ([]byte, error) {
	buf := bufio.NewReader(r)

	buf.ReadByte() // skip " "

	b, err := buf.ReadSlice(0x00)
	if err != nil {
		return nil, err
	}
	return b[:len(b)-1], nil
}

func Parse(r io.Reader) (*Object, error) {
	var obj Object
	b, err := parseType(r)
	if err != nil {
		return nil, err
	}
	obj.Type = string(b)

	b, err = parseSize(r)
	if err != nil {
		return nil, err
	}
	obj.Size, err = strconv.Atoi(string(b))
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func main() {
	if len(os.Args) != 2 {
		panic("one argument required")
	}

	p := filepath.Join(".git", "objects", os.Args[1][:2], os.Args[1][2:])
	fmt.Println(p)
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

	obj, err := Parse(r)
	if err != nil {
		panic(err)
	}

	pp.Println(obj)
}
