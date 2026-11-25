package content

import (
	"fmt"
	"image/color"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/npillmayer/css-box-layout/layout"
	boxtext "github.com/npillmayer/css-box-layout/text"
	"github.com/npillmayer/w3c-dom/style/textstyle"
)

// GroupInput is detached, backend-neutral input for one normalized group.
// Every range addresses Text's processed UTF-8 bytes. Source ranges instead
// address individual decoded text nodes, never HTML markup or rune offsets.
// Slices are independently owned; only immutable Box handles retain the source.
// TextStyle remains the DOM snapshot (including line-height and hyphens); no
// typography conversion or policy admission happens here. Paint is also supplied
// separately so a later shaping bridge need not use color as a style boundary.
type GroupInput struct {
	// Events maps source annotations to processed bytes without inserting material.
	Events []GroupEvent
	GroupInfo
	Text        string
	Styles      []TextStyleSpan
	Paint       []TextPaintSpan
	Hyphens     []HyphenationSpan // Contributing text only, never atomic markers.
	Isolates    []IsolateSpan     // Structural postorder, including equal nested ranges.
	TextSources []TextSourceSpan
	Occurrences []AtomicOccurrence
}

// TextStyleSpan describes source typography and text policy in processed bytes.
type TextStyleSpan struct {
	Start, End uint64
	Style      textstyle.Style
}

// TextPaintSpan supplies foreground paint separately from shaping input.
type TextPaintSpan struct {
	Start, End uint64
	Color      color.NRGBA
}

// HyphenationSpan retains the first contributing source range, even for a space
// collapsed across several sources. Omitted source policies remain in TextSources.
type HyphenationSpan struct {
	Start, End uint64
	Policy     string
	Box        Box
	Source     SourceRef
	Range      boxtext.TextRange
}

// IsolateSpan describes a nonempty structural bidi scope in processed bytes.
type IsolateSpan struct {
	Start, End uint64
	Direction  textstyle.Direction
	Source     SourceRef
}

// TextSourceSpan maps processed bytes to decoded source bytes. Multiple collapsed
// source fragments may map to the same space; omitted fragments map to no bytes.
type TextSourceSpan struct {
	Start, End uint64
	Box        Box
	Source     SourceRef
	Range      boxtext.TextRange
	Kind       string // Empty for identity; otherwise collapsed or omitted.
	Style      textstyle.Style
}

// AtomicOccurrence identifies an inserted marker. ID is one-based occurrence
// order within this group, independent of BoxID.
// Authored U+FFFC is ordinary text and never creates an occurrence.
type AtomicOccurrence struct {
	ID         uint64
	Start, End uint64
	Box        Box
	Source     SourceRef
}

// Extraction failures become public stage diagnostics at the Prepare boundary.
// No partial group input accompanies an error.
type extractionError struct {
	Group, Owner Box
	Source       SourceRef
	Range        boxtext.TextRange
	HasRange     bool
	Err          error
}

func (e *extractionError) Error() string { return fmt.Sprintf("content extraction: %v", e.Err) }
func (e *extractionError) Unwrap() error { return e.Err }

type extractionPiece struct {
	start, end uint64
	box        Box
	source     SourceRef
	style      textstyle.Style
	nodeStart  uint64
	atomic     bool
}

type extractionScope struct {
	start, end uint64
	source     SourceRef
	direction  textstyle.Direction
}

// extractGroup consumes the existing normalized group index, not source DOM
// children. Tree order is residual first then global branch order; group indices
// follow InlineGroups preorder. It does no font, service, bank or layout work.
func (s *snapshot) extractGroup(tree TreeID, index int) (*GroupInput, error) {
	if s == nil || tree == 0 || uint64(tree) >= uint64(len(s.trees)) || index < 0 || index >= len(s.trees[tree].groups) {
		return nil, &extractionError{Err: fmt.Errorf("unknown normalized group")}
	}
	info := s.trees[tree].groups[index]
	info.Atomics = slices.Clone(info.Atomics)
	g := &GroupInput{GroupInfo: info}
	fail := func(source SourceRef, ref *boxtext.TextRange, err error) error {
		e := &extractionError{Group: info.Group, Owner: info.Owner, Source: source, Err: err}
		if ref != nil {
			e.Range, e.HasRange = *ref, true
		}
		return e
	}
	root := info.Group.node()
	if root == nil || root.Box != layout.BoxAnonymousInline {
		return nil, fail(info.Source, nil, fmt.Errorf("missing normalized group"))
	}
	var raw strings.Builder
	var pieces []extractionPiece
	var scopes []extractionScope
	var walk func(*layout.LayoutNode) error
	walk = func(n *layout.LayoutNode) error {
		if n == nil || n.NodeID == 0 || uint64(n.NodeID) >= uint64(len(s.sources)) || s.trees[tree].nodes[n.BoxID] != n {
			return fail(info.Source, nil, fmt.Errorf("invalid normalized inline source"))
		}
		source := s.sources[n.NodeID]
		if source.Tree != tree || !source.HasTextStyle {
			return fail(source.Source, nil, fmt.Errorf("missing source text context"))
		}
		start := uint64(raw.Len())
		p := extractionPiece{start: start, box: Box{s, tree, n.BoxID}, source: source.Source, style: source.TextStyle}
		switch n.Box {
		case layout.BoxText:
			if layout.NodeID(n.Text.Source) != n.NodeID {
				return fail(source.Source, &n.Text.Range, fmt.Errorf("text reference belongs to another source"))
			}
			text, err := s.readText(n.Text)
			if err != nil {
				return fail(source.Source, &n.Text.Range, err)
			}
			if !utf8.ValidString(text) {
				return fail(source.Source, &n.Text.Range, fmt.Errorf("invalid UTF-8 text source"))
			}
			raw.WriteString(text)
			p.nodeStart, p.end = n.Text.Range.Start, uint64(raw.Len())
			pieces = append(pieces, p)
		case layout.BoxInlineBlock:
			raw.WriteRune('\uFFFC')
			p.atomic, p.end = true, uint64(raw.Len())
			pieces = append(pieces, p)
		case layout.BoxInline:
			for _, child := range n.Children {
				if err := walk(child); err != nil {
					return err
				}
			}
		default:
			return fail(source.Source, nil, fmt.Errorf("non-inline child in normalized group"))
		}
		// Text nodes carry their parent's complete style, but do not introduce
		// another isolate. Atomic scopes contain only their marker, not interiors.
		if n.Box != layout.BoxText && source.TextStyle.UnicodeBidi == "isolate" {
			scopes = append(scopes, extractionScope{start, uint64(raw.Len()), source.Source, source.TextStyle.Direction})
		}
		return nil
	}
	for _, child := range root.Children {
		if err := walk(child); err != nil {
			return nil, err
		}
	}

	text, transforms := boxtext.NormalizeNormal(raw.String())
	g.Text = text
	pi := 0
	for _, tr := range transforms {
		for pi < len(pieces) && pieces[pi].end <= tr.SourceStart {
			pi++
		}
		for j := pi; j < len(pieces) && pieces[j].start < tr.SourceEnd; j++ {
			p := pieces[j]
			from, to := max(tr.SourceStart, p.start), min(tr.SourceEnd, p.end)
			start, end := tr.Start, tr.End
			if tr.Kind == "" {
				start = tr.Start + from - tr.SourceStart
				end = start + to - from
			}
			ref := boxtext.TextRange{Start: p.nodeStart + from - p.start, End: p.nodeStart + to - p.start}
			if p.atomic {
				if tr.Kind != "" || end-start != 3 || text[start:end] != "\uFFFC" {
					return nil, fail(p.source, nil, fmt.Errorf("transformed atomic marker"))
				}
				if len(g.Occurrences) >= len(info.Atomics) || p.box != info.Atomics[len(g.Occurrences)] {
					return nil, fail(p.source, nil, fmt.Errorf("atomic occurrence differs from group index"))
				}
				g.Occurrences = append(g.Occurrences, AtomicOccurrence{uint64(len(g.Occurrences) + 1), start, end, p.box, p.source})
			} else {
				g.appendTextSource(TextSourceSpan{start, end, p.box, p.source, ref, tr.Kind, p.style})
			}
			// First-unit ownership applies consistently to style, paint, policy
			// and isolate scopes. All raw contributors still have source spans.
			if start < end && (tr.Kind == "" || tr.Kind == "collapsed" && from == tr.SourceStart) {
				g.appendStyle(start, end, p.style)
				if !p.atomic {
					g.Hyphens = append(g.Hyphens, HyphenationSpan{start, end, p.style.Hyphens, p.box, p.source, ref})
				}
			}
		}
	}
	if len(g.Occurrences) != len(info.Atomics) {
		return nil, fail(info.Source, nil, fmt.Errorf("missing indexed atomic occurrence"))
	}
	for _, scope := range scopes {
		start, end, found := projectExtractionScope(transforms, scope.start, scope.end)
		if found {
			g.Isolates = append(g.Isolates, IsolateSpan{start, end, scope.direction, scope.source})
		}
	}
	return g, nil
}

func (g *GroupInput) appendStyle(start, end uint64, style textstyle.Style) {
	if n := len(g.Styles); n > 0 && g.Styles[n-1].End == start && g.Styles[n-1].Style == style {
		g.Styles[n-1].End = end
	} else {
		g.Styles = append(g.Styles, TextStyleSpan{start, end, style})
	}
	if n := len(g.Paint); n > 0 && g.Paint[n-1].End == start && g.Paint[n-1].Color == style.Foreground {
		g.Paint[n-1].End = end
	} else {
		g.Paint = append(g.Paint, TextPaintSpan{start, end, style.Foreground})
	}
}

func (g *GroupInput) appendTextSource(span TextSourceSpan) {
	if n := len(g.TextSources); n > 0 {
		last := &g.TextSources[n-1]
		if last.Box == span.Box && last.Source == span.Source && last.Style == span.Style && last.Range.End == span.Range.Start && last.Kind == span.Kind &&
			(span.Kind == "" && last.End == span.Start || span.Kind != "" && last.Start == span.Start && last.End == span.End) {
			last.End, last.Range.End = span.End, span.Range.End
			return
		}
	}
	g.TextSources = append(g.TextSources, span)
}

// Project structural scopes, including split inline fragments, onto the same
// transformed bytes as text/atomics. Collapsed bytes belong only to scopes
// containing the first raw contributing byte. Empty scopes introduce no markup.
func projectExtractionScope(transforms []boxtext.Span, from, to uint64) (start, end uint64, found bool) {
	i := sort.Search(len(transforms), func(i int) bool { return transforms[i].SourceEnd > from })
	for _, tr := range transforms[i:] {
		if tr.SourceStart >= to {
			break
		}
		if tr.Kind == "omitted" {
			continue
		}
		a, b := tr.Start, tr.End
		if tr.Kind == "" {
			a = tr.Start + max(from, tr.SourceStart) - tr.SourceStart
			b = tr.Start + min(to, tr.SourceEnd) - tr.SourceStart
		} else if tr.SourceStart < from {
			continue
		}
		if a < b {
			if !found {
				start, found = a, true
			}
			end = b
		}
	}
	return
}
