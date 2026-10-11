package lichdiff

import (
	"slices"
	"testing"

	"github.com/lichdotdev/godemirror"
	"github.com/omartelo/lich/native/lichclient"
)

func TestRevertLinesNamesSidesAsTheBackendTakesThem(t *testing.T) {
	c := godemirror.Chunk{Lines: []godemirror.ChangedLine{
		{Side: godemirror.OldSide, Line: 4, Text: "b"},
		{Side: godemirror.NewSide, Line: 4, Text: "B"},
	}}
	want := []lichclient.RevertLine{{Side: "old", Line: 4, Text: "b"}, {Side: "new", Line: 4, Text: "B"}}
	if got := RevertLines(c); !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
