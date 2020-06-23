package scdt

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion_String(t *testing.T) {
	v := Version{
		'v', 1, 3, 5,
	}
	t.Log(strings.Compare(v.String(), "v1.3.5"))
}

func TestParseVersion(t *testing.T) {
	version, err := ParseVersion("v1.3.5")
	if err != nil {
		t.Fatal(err)
	}
	b := [4]byte{
		'v', 1, 3, 5,
	}

	t.Log(bytes.Compare(version[:], b[:]))

	version1, err := ParseVersion("v1.3")
	if err != nil {
		t.Fatal(err)
	}
	b1 := [4]byte{
		'v', 1, 3,
	}

	t.Log(bytes.Compare(version1[:], b1[:]))

}

func TestVersion_Compare(t *testing.T) {
	type args struct {
		version Version
	}
	tests := []struct {
		name string
		v    Version
		args args
		want int
	}{
		// TODO: Add test cases.
		{
			name: "t1",
			v:    Version{'v', 0, 0, 1},
			args: args{
				version: Version{'v', 0, 0, 1},
			},
			want: 0,
		},
		{
			name: "t2",
			v:    Version{'v', 0, 0, 1},
			args: args{
				version: Version{'v', 0, 0, 2},
			},
			want: -3,
		},
		{
			name: "t3",
			v:    Version{'v', 0, 1, 1},
			args: args{
				version: Version{'v', 0, 2, 2},
			},
			want: -2,
		},
		{
			name: "t4",
			v:    Version{'v', 1, 1, 1},
			args: args{
				version: Version{'v', 2, 2, 2},
			},
			want: -1,
		},
		{
			name: "t5",
			v:    Version{'v', 2, 2, 2},
			args: args{
				version: Version{'v', 1, 1, 1},
			},
			want: 1,
		},
		{
			name: "t6",
			v:    Version{'v', 0, 2, 2},
			args: args{
				version: Version{'v', 0, 1, 1},
			},
			want: 2,
		},
		{
			name: "t7",
			v:    Version{'v', 0, 0, 2},
			args: args{
				version: Version{'v', 0, 0, 1},
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.Compare(tt.args.version); got != tt.want {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
