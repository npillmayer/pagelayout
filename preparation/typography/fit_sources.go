package typography

import (
	"slices"

	boxtext "github.com/npillmayer/css-box-layout/text"
	"github.com/npillmayer/pagelayout/content"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

// FragmentSource joins one indivisible rendered coverage record to original
// decoded source bytes. Start/End are its intersection with this source span in
// processed group bytes. Coverage keeps the whole cluster/replacement partition;
// never infer replacement-local or per-character glyph coordinates from Range.
type FragmentSource struct {
	Coverage   paragraph.SourceCoverage
	Box        content.Box
	Source     content.SourceRef
	Start, End uint64
	Range      boxtext.TextRange
}

// FragmentEvent associates a source annotation with rendered coverage, without
// inserting marker material or activating a flow. Line is fragment-local; -1
// and HasCoverage=false denote structural empty-group affinity. The page layer
// deduplicates an event spanning several groups by source identity and kind.
type FragmentEvent struct {
	Event       content.GroupEvent
	Coverage    paragraph.SourceCoverage
	HasCoverage bool
	Line        int
}

func mapFragmentSources(g content.GroupInput, coverage []paragraph.SourceCoverage) ([]FragmentSource, error) {
	var out []FragmentSource
	for _, c := range coverage {
		if c.Source.Start >= c.Source.End || c.Source.End > uint64(len(g.Text)) || c.Line < 0 {
			return nil, content.ErrPreparationProtocol
		}
		found := false
		if c.Kind == paragraph.AtomicCoverage {
			for _, a := range g.Occurrences {
				if uint64(c.Atomic) == a.ID && c.Source.Start == a.Start && c.Source.End == a.End {
					out = append(out, FragmentSource{Coverage: c, Box: a.Box, Source: a.Source, Start: a.Start, End: a.End})
					found = true
					break
				}
			}
		} else {
			for _, s := range g.TextSources {
				start, end := max(c.Source.Start, s.Start), min(c.Source.End, s.End)
				if start >= end {
					continue
				}
				rangeInSource := s.Range
				if s.Kind == "" {
					rangeInSource.Start += start - s.Start
					rangeInSource.End -= s.End - end
				}
				out = append(out, FragmentSource{c, s.Box, s.Source, start, end, rangeInSource})
				found = true
			}
		}
		if !found {
			return nil, content.ErrPreparationProtocol
		}
	}
	return out, nil
}

type eventKey struct {
	source content.SourceRef
	kind   content.SourceEventKind
}

func mapFragmentEvents(g content.GroupInput, info paragraph.PrefixInfo, coverage []paragraph.SourceCoverage, seen []eventKey) []FragmentEvent {
	var out []FragmentEvent
	keys := slices.Clone(seen)
	for _, e := range g.Events {
		key := eventKey{e.Source, e.Kind}
		if slices.Contains(keys, key) {
			continue
		}
		var chosen paragraph.SourceCoverage
		found := false
		for _, c := range coverage {
			matches := false
			switch e.Affinity {
			case content.AffinityVisible:
				matches = c.Source.Start < e.End && e.Start < c.Source.End
			case content.AffinityFollowing:
				matches = c.Source.End > e.Start
			case content.AffinityPreceding:
				matches = info.To.Offset >= e.Start && c.Source.Start < e.Start
			}
			if matches {
				chosen, found = c, true
				if e.Affinity != content.AffinityPreceding {
					break
				}
			}
		}
		if found {
			out = append(out, FragmentEvent{e, chosen, true, chosen.Line})
			keys = append(keys, key)
		} else if e.Affinity == content.AffinityStructural && g.Text == "" && info.Status == paragraph.FitComplete {
			out = append(out, FragmentEvent{Event: e, Line: -1})
			keys = append(keys, key)
		}
	}
	return out
}
