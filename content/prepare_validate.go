package content

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/npillmayer/css-box-layout/layout"
	boxtext "github.com/npillmayer/css-box-layout/text"
	"github.com/npillmayer/w3c-dom/style/textstyle"
)

// Validate the neutral extraction contract before invoking any backend. CSS
// grammar/profile checks belong to loading; backend service/policy admission
// belongs to Preflight. All coordinates here are processed/decoded UTF-8 bytes.
func validateGroupInput(g GroupInput) error {
	bad := func(message string) error { return fmt.Errorf("%w: %s", ErrInvalidPreparation, message) }
	s := g.Group.owner
	group, gok := g.Group.Info()
	owner, ook := g.Owner.Info()
	if !gok || !ook || g.Owner.owner != s || g.Owner.tree != g.Group.tree || group.Kind != layout.BoxAnonymousInline || !layout.IsBlockContainer(owner.Kind) ||
		g.Source != owner.Context || g.TextStyle != owner.TextStyle || !utf8.ValidString(g.Text) {
		return bad("invalid group/owner/text context")
	}
	qualified := func(b Box) bool { return b.owner == s && b.tree == g.Group.tree && b.Valid() }
	sourceOK := func(ref SourceRef) bool {
		return ref.NodeID > 0 && uint64(ref.NodeID) < uint64(len(s.sources)) && s.sources[ref.NodeID].Source == ref && s.sources[ref.NodeID].Tree == g.Group.tree
	}
	if g.ParentAtomic != (Box{}) {
		p, ok := g.ParentAtomic.Info()
		if !ok || !qualified(g.ParentAtomic) || p.Kind != layout.BoxInlineBlock {
			return bad("invalid parent atomic")
		}
	}
	end := uint64(0)
	for _, span := range g.Styles {
		if span.Start != end || span.Start == span.End || !validByteRange(g.Text, span.Start, span.End) {
			return bad("incomplete style coverage")
		}
		end = span.End
	}
	if end != uint64(len(g.Text)) {
		return bad("missing styles")
	}
	end = 0
	for _, span := range g.Paint {
		if span.Start != end || span.Start == span.End || !validByteRange(g.Text, span.Start, span.End) {
			return bad("incomplete paint coverage")
		}
		end = span.End
	}
	if end != uint64(len(g.Text)) {
		return bad("missing paint")
	}
	// One byte of temporary coverage per text byte: ordinary text, shared
	// collapsed space, atomic marker, then policy-covered text. No state escapes.
	coverage := make([]byte, len(g.Text))
	if len(g.Atomics) != len(g.Occurrences) {
		return bad("incomplete atomic coverage")
	}
	end = 0
	for i, atom := range g.Occurrences {
		info, ok := atom.Box.Info()
		if !ok || !qualified(atom.Box) || info.Kind != layout.BoxInlineBlock || atom.Box != g.Atomics[i] || atom.Source != info.Source || atom.ID != uint64(i+1) ||
			!validByteRange(g.Text, atom.Start, atom.End) || atom.Start < end || g.Text[atom.Start:atom.End] != "\uFFFC" {
			return bad("invalid atomic occurrence")
		}
		for k := atom.Start; k < atom.End; k++ {
			coverage[k] = 3
		}
		end = atom.End
	}
	for _, span := range g.TextSources {
		info, ok := span.Box.Info()
		if !ok || !qualified(span.Box) || info.Kind != layout.BoxText || span.Source != info.Source || span.Style != info.TextStyle ||
			!validByteRange(g.Text, span.Start, span.End) || span.Range.Start < info.Text.Range.Start || span.Range.End > info.Text.Range.End {
			return bad("invalid text-source correspondence")
		}
		raw := s.texts[info.Text.Source].Text
		if !validByteRange(raw, span.Range.Start, span.Range.End) || span.Range.Start == span.Range.End {
			return bad("invalid decoded text range")
		}
		part, out := raw[span.Range.Start:span.Range.End], g.Text[span.Start:span.End]
		switch span.Kind {
		case "":
			if part != out {
				return bad("identity source mapping")
			}
		case "collapsed", "omitted":
			if strings.Trim(part, " \t\r\n\f") != "" || span.Kind == "collapsed" && out != " " || span.Kind == "omitted" && out != "" {
				return bad("whitespace source mapping")
			}
		default:
			return bad("unknown source transform")
		}
		for k := span.Start; k < span.End; k++ {
			if coverage[k] != 0 && !(coverage[k] == 2 && span.Kind == "collapsed") {
				return bad("overlapping source/atomic mapping")
			}
			coverage[k] = 1
			if span.Kind == "collapsed" {
				coverage[k] = 2
			}
		}
	}
	end = 0
	for _, span := range g.Hyphens {
		info, ok := span.Box.Info()
		if !ok || !qualified(span.Box) || info.Kind != layout.BoxText || span.Source != info.Source || span.Policy != info.TextStyle.Hyphens ||
			!validByteRange(g.Text, span.Start, span.End) || span.Start == span.End || span.Start < end ||
			span.Range.Start < info.Text.Range.Start || span.Range.End > info.Text.Range.End {
			return bad("invalid policy correspondence")
		}
		raw := s.texts[info.Text.Source].Text
		if !validByteRange(raw, span.Range.Start, span.Range.End) || span.Range.Start == span.Range.End {
			return bad("invalid policy source range")
		}
		for k := span.Start; k < span.End; k++ {
			if coverage[k] != 1 && coverage[k] != 2 {
				return bad("policy outside contributing text")
			}
			coverage[k] = 4
		}
		end = span.End
	}
	for _, c := range coverage {
		if c != 3 && c != 4 {
			return bad("missing source or policy coverage")
		}
	}
	isolates := slices.Clone(g.Isolates)
	slices.SortStableFunc(isolates, func(a, b IsolateSpan) int {
		if a.Start < b.Start || a.Start == b.Start && a.End > b.End {
			return -1
		}
		if a.Start == b.Start && a.End == b.End {
			return 0
		}
		return 1
	})
	var stack []uint64
	for _, scope := range isolates {
		if !sourceOK(scope.Source) || !validByteRange(g.Text, scope.Start, scope.End) || scope.Start == scope.End || scope.Direction > textstyle.RightToLeft {
			return bad("invalid isolate range")
		}
		for len(stack) > 0 && scope.Start >= stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 && scope.End > stack[len(stack)-1] {
			return bad("crossing isolate scopes")
		}
		stack = append(stack, scope.End)
	}
	return nil
}

func validByteRange(text string, start, end uint64) bool {
	boundary := func(i uint64) bool { return i == uint64(len(text)) || utf8.RuneStart(text[i]) }
	return start <= end && end <= uint64(len(text)) && boundary(start) && boundary(end)
}

// Validate source-event coordinates separately from shaping/style coverage.
func validateGroupEvents(g GroupInput) error {
	for _, event := range g.Events {
		s := g.Group.owner
		validSource := func(ref SourceRef) bool {
			return ref.NodeID > 0 && uint64(ref.NodeID) < uint64(len(s.sources)) && s.sources[ref.NodeID].Source == ref && s.sources[ref.NodeID].Tree == g.Group.tree
		}
		if !validSource(event.Source) || !validSource(event.CoverageSource) || !validByteRange(g.Text, event.Start, event.End) || event.Kind != SourceFlowAnchor {
			return fmt.Errorf("%w: invalid source-event coordinates", ErrInvalidPreparation)
		}
		if !event.Atomic && event.Affinity != AffinityStructural {
			raw, ok := s.texts[boxtext.TextSourceID(event.CoverageSource.NodeID)]
			if !ok || !validByteRange(raw.Text, event.Range.Start, event.Range.End) {
				return fmt.Errorf("%w: invalid event source range", ErrInvalidPreparation)
			}
		}
		switch event.Affinity {
		case AffinityVisible:
			if event.Start == event.End {
				return fmt.Errorf("%w: empty visible event coverage", ErrInvalidPreparation)
			}
		case AffinityFollowing, AffinityPreceding, AffinityStructural:
			if event.Start != event.End {
				return fmt.Errorf("%w: nonempty boundary event", ErrInvalidPreparation)
			}
		default:
			return fmt.Errorf("%w: invalid event affinity", ErrInvalidPreparation)
		}
	}
	return nil
}
