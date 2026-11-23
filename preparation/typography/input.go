package typography

import (
	"fmt"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing/paragraph"
	typestyle "github.com/npillmayer/typography/style"
	"github.com/npillmayer/uax/bidi"
	domdimen "github.com/npillmayer/w3c-dom/dimen"
	"github.com/npillmayer/w3c-dom/style/textstyle"
)

type mappedInput struct {
	owner      typestyle.Style
	styles     []typestyle.Span
	anchors    []contracts.AtomicAnchor
	markup     bidi.OutOfLineBidiMarkup
	policy     khipu.HyphenationMode
	lineLayout paragraph.LineLayout
}

func unsupported(message string) error {
	return fmt.Errorf("%w: %s", khipu.ErrUnsupportedPolicy, message)
}

func mapStyle(s textstyle.Style) (typestyle.Style, error) {
	// Explicit physical conversion: DOM PX is 3/4 BP; khipu PX aliases BP.
	// Check the rounded value before narrowing to typography's finite DU domain.
	size := math.Round(float64(s.FontSize) * float64(dimen.BP) / float64(domdimen.BP))
	if size < 1 || size >= float64(dimen.Fil) || math.IsNaN(size) || math.IsInf(size, 0) {
		return typestyle.Style{}, unsupported("font-size outside typography's finite range")
	}
	if s.WhiteSpace != "normal" || s.LineHeight.Kind != textstyle.NormalLineHeight || s.UnicodeBidi != "normal" && s.UnicodeBidi != "isolate" {
		return typestyle.Style{}, unsupported("text requires normal whitespace/line-height and normal/isolate bidi")
	}
	dir, err := mapDirection(s.Direction)
	if err != nil {
		return typestyle.Style{}, err
	}
	// Foreground paint remains in GroupInput.Paint, outside shaping styles.
	return typestyle.Style{Family: s.Family, EmSize: dimen.DU(size), Weight: s.Weight, Slant: s.Slant, Language: s.Language, Direction: dir, WhiteSpace: s.WhiteSpace, UnicodeBidi: s.UnicodeBidi}, nil
}

func mapDirection(d textstyle.Direction) (bidi.Direction, error) {
	switch d {
	case textstyle.LeftToRight:
		return bidi.LeftToRight, nil
	case textstyle.RightToLeft:
		return bidi.RightToLeft, nil
	}
	return bidi.LeftToRight, unsupported("unknown text direction")
}

func (b *Backend) mapInput(g content.GroupInput) (mappedInput, error) {
	var m mappedInput
	if !g.Group.Valid() || !g.Owner.Valid() || !utf8.ValidString(g.Text) {
		return m, groupError(g, content.StagePreflight, fmt.Errorf("%w: invalid group input", contracts.ErrIllegalArguments))
	}
	var err error
	m.owner, err = mapStyle(g.TextStyle)
	if err != nil {
		return m, groupError(g, content.StagePreflight, err)
	}
	m.styles = make([]typestyle.Span, len(g.Styles))
	m.lineLayout.Default = paragraph.LineHeight{Kind: paragraph.NormalHeight}
	for i, s := range g.Styles {
		style, err := mapStyle(s.Style)
		if err != nil {
			return m, inputError(g, s.Start, err)
		}
		m.styles[i] = typestyle.Span{Start: s.Start, End: s.End, Style: style}
		m.lineLayout.Runs = append(m.lineLayout.Runs, paragraph.LineHeightSpan{Start: s.Start, End: s.End, Height: m.lineLayout.Default})
	}
	if err := typestyle.ValidateSpans(g.Text, m.styles); err != nil {
		return m, groupError(g, content.StagePreflight, err)
	}
	policy := g.TextStyle.Hyphens
	if len(g.Hyphens) > 0 {
		policy = g.Hyphens[0].Policy
	}
	for _, s := range g.Hyphens {
		if s.Policy != policy || s.Policy != "none" && s.Policy != "manual" {
			return m, inputError(g, s.Start, unsupported("mixed or automatic hyphenation"))
		}
	}
	switch policy {
	case "none":
		m.policy = khipu.NoHyphenation
	case "manual":
		if nilService(b.config.Hyphenator) {
			err := fmt.Errorf("%w: hyphens:manual requires an explicit-marker hyphenator", contracts.ErrIllegalArguments)
			if len(g.Hyphens) > 0 {
				return m, inputError(g, g.Hyphens[0].Start, err)
			}
			return m, groupError(g, content.StagePreflight, err)
		}
		m.policy = khipu.ExplicitHyphenation
	default:
		return m, groupError(g, content.StagePreflight, unsupported("automatic or unknown hyphenation"))
	}
	seen := make(map[contracts.AtomicID]bool, len(g.Occurrences))
	for i, a := range g.Occurrences {
		id := contracts.AtomicID(a.ID)
		if id == 0 || seen[id] || !byteRange(g.Text, a.Start, a.End) || a.End-a.Start != 3 || g.Text[a.Start:a.End] != "\uFFFC" || i > 0 && a.Start <= g.Occurrences[i-1].Start {
			return m, inputError(g, a.Start, fmt.Errorf("%w: invalid atomic anchor", contracts.ErrIllegalArguments))
		}
		seen[id] = true
		m.anchors = append(m.anchors, contracts.AtomicAnchor{ID: id, ByteOffset: a.Start})
	}
	m.markup, err = mapIsolates(g)
	return m, err
}

// The upstream callback supports one close and one open per byte offset.
// Reject coincident nested events before service work; never inject characters
// or shift offsets to work around that limitation.
func mapIsolates(g content.GroupInput) (bidi.OutOfLineBidiMarkup, error) {
	ordered := slices.Clone(g.Isolates)
	slices.SortStableFunc(ordered, func(a, b content.IsolateSpan) int {
		if a.Start < b.Start || a.Start == b.Start && a.End > b.End {
			return -1
		}
		if a.Start == b.Start && a.End == b.End {
			return 0
		}
		return 1
	})
	boundaries := make(map[uint64]int)
	var stack []uint64
	for _, s := range ordered {
		fail := func(err error) (bidi.OutOfLineBidiMarkup, error) {
			e := groupError(g, content.StagePreflight, err)
			e.Source = s.Source
			return nil, e
		}
		if !byteRange(g.Text, s.Start, s.End) || s.Start == s.End {
			return fail(fmt.Errorf("%w: invalid isolate range", contracts.ErrIllegalArguments))
		}
		for len(stack) > 0 && s.Start >= stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 && s.End > stack[len(stack)-1] {
			return fail(unsupported("crossing bidi isolates"))
		}
		stack = append(stack, s.End)
		dir, err := mapDirection(s.Direction)
		if err != nil {
			return fail(err)
		}
		open := bidi.MarkupLRI
		if dir == bidi.RightToLeft {
			open = bidi.MarkupRLI
		}
		if boundaries[s.Start]&^bidi.MarkupPDI != 0 || boundaries[s.End]&bidi.MarkupPDI != 0 {
			return fail(unsupported("coincident nested bidi boundaries"))
		}
		boundaries[s.Start] |= open
		boundaries[s.End] |= bidi.MarkupPDI
	}
	return func(pos uint64) int { return boundaries[pos] }, nil
}

func byteRange(text string, start, end uint64) bool {
	boundary := func(i uint64) bool { return i == uint64(len(text)) || utf8.RuneStart(text[i]) }
	return start <= end && end <= uint64(len(text)) && boundary(start) && boundary(end)
}
