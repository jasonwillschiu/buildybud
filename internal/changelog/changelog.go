package changelog

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var headingRe = regexp.MustCompile(`^#\s+([0-9]+\.[0-9]+\.[0-9]+)\s+-\s+(.+)$`)

const ExpectedFormat = "# <version> - <summary>\\n- bullet"

type Entry struct {
	Version string
	Summary string
}

type ParseError struct {
	Path string
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse %s: %s", e.Path, e.Msg)
}

func ParseLatest(path string) (*Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseLatestContent(string(data), path)
}

func ParseLatestContent(content, path string) (*Entry, error) {
	s := bufio.NewScanner(strings.NewReader(content))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		m := headingRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		return &Entry{Version: m[1], Summary: m[2]}, nil
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return nil, &ParseError{Path: path, Msg: "no release heading found"}
}
