package scdt

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version ...
type Version [4]byte

// String ...
func (v Version) String() string {
	return fmt.Sprintf("%s%d.%d.%d", string(v[0]), v[1], v[2], v[3])
}

// ParseVersion ...
func ParseVersion(s string) (Version, error) {
	var v Version
	if s[0] != 'v' {
		return v, errors.New("wrong start code")
	}
	v[0] = 'v'
	splited := strings.Split(s[1:], ".")
	sz := len(splited)
	if sz > 3 {
		return v, fmt.Errorf("wrong version size(%d)", sz)
	}
	for i := 0; i < sz; i++ {
		parseInt, err := strconv.ParseInt(splited[i], 10, 8)
		if err != nil {
			return [4]byte{}, fmt.Errorf("parse int failed:%w", err)
		}
		v[i+1] = byte(parseInt)
	}
	return v, nil
}

// Compare ...
func (v Version) Compare(version Version) int {
	max := len(v)
	for i := 1; i < max; i++ {
		if v[i] > version[i] {
			return i
		} else if v[i] < version[i] {
			return -i
		}
	}
	return 0
}
