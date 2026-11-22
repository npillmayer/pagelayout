package template

import (
	"image/color"
	"slices"

	"github.com/npillmayer/css-box-layout/layout"
)

// Rect is a rectangle in page-local CSS pixels, with a zero-origin page.
type Rect = layout.Rect

// Edges contains top, right, bottom and left widths in CSS pixels.
type Edges = layout.Edges

// FrameID is a one-based source-order identity scoped to a template, not an HTML id.
type FrameID uint64

// SourceRef identifies a node in the parsed collection, including HTML repairs.
// NodeID is collection-local; Path is a node path, not a byte or line location.
// Name is the diagnostic label passed to Load, never a resource base URL.
type SourceRef struct {
	Name, TemplateID, Path string
	NodeID                 uint64
}

// BorderStyle is a resolved border style supported by the template profile.
type BorderStyle string

const (
	BorderNone   BorderStyle = "none"
	BorderHidden BorderStyle = "hidden"
	BorderSolid  BorderStyle = "solid"
)

// BorderPaint contains a resolved style and non-premultiplied sRGB color.
// Effective widths are in FrameInfo.Border; page border widths are always zero.
type BorderPaint struct {
	Style BorderStyle
	Color color.NRGBA
}

// BorderPaints holds the four physical border paints as independent values.
type BorderPaints struct{ Top, Right, Bottom, Left BorderPaint }

// Paint contains resolved solid paint values; it requires no later CSS cascade.
type Paint struct {
	Background color.NRGBA
	Borders    BorderPaints
}

// PageInfo describes the page box. Width/Height are positive CSS-pixel sizes;
// margin, padding and effective borders are zero in the initial profile.
type PageInfo struct {
	Width, Height float32
	Paint         Paint
	Source        SourceRef
}

// FrameInfo is a detached receiving-frame description. Both rectangles are in
// page coordinates: ContentBox already includes the frame's position and edges.
// ID is template-local and Flow is a decoded, case-sensitive channel name.
// All edges are effective CSS-pixel values; Margin is zero in this profile.
type FrameInfo struct {
	ID                      FrameID
	Flow                    string
	BorderBox, ContentBox   Rect
	Margin, Padding, Border Edges
	Paint                   Paint
	Source                  SourceRef
}

// Catalog is a read-only collection of fully compiled templates. Selection and
// inspection require no parsing, styling or layout. Read-only access is safe
// concurrently. Its zero value is an empty catalog; Load never returns an empty
// successful catalog. Copying a Catalog copies a handle to immutable storage.
type Catalog struct {
	ids  []string
	byID map[string]*compiledTemplate
}

// Template is a read-only handle to a page and its ordered child frames.
// Copying or overwriting a returned handle does not modify its catalog.
type Template struct{ data *compiledTemplate }

type compiledTemplate struct {
	id     string
	page   PageInfo
	frames []FrameInfo
	flows  map[string][]int // Positions in frames, always in source order.
}

// IDs returns a copy of the template IDs in definition order.
func (c *Catalog) IDs() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.ids)
}

// Lookup selects an exact, case-sensitive ID and returns a fresh immutable-data
// handle. A missing ID returns nil, false, including for a nil or empty catalog.
func (c *Catalog) Lookup(id string) (*Template, bool) {
	if c == nil {
		return nil, false
	}
	t, ok := c.byID[id]
	if !ok {
		return nil, false
	}
	return &Template{data: t}, true
}

// ID returns this template's collection ID, or empty for a nil/zero handle.
func (t *Template) ID() string {
	if t == nil || t.data == nil {
		return ""
	}
	return t.data.id
}

// Page returns a value copy of the page description, or zero for a nil/zero handle.
func (t *Template) Page() PageInfo {
	if t == nil || t.data == nil {
		return PageInfo{}
	}
	return t.data.page
}

// Frames returns a copy of the direct child frames in source order.
func (t *Template) Frames() []FrameInfo {
	if t == nil || t.data == nil {
		return nil
	}
	return slices.Clone(t.data.frames)
}

// FramesForFlow returns copied frames for a decoded, case-sensitive flow name,
// retaining source order even when it differs from visual order. Missing flows
// and nil/zero handles return an empty slice. No content document is required.
func (t *Template) FramesForFlow(name string) []FrameInfo {
	if t == nil || t.data == nil {
		return nil
	}
	indices := t.data.flows[name]
	if len(indices) == 0 {
		return nil
	}
	frames := make([]FrameInfo, len(indices))
	for i, index := range indices {
		frames[i] = t.data.frames[index]
	}
	return frames
}

func publicSource(s sourceRef) SourceRef {
	return SourceRef{Name: s.name, TemplateID: s.templateID, Path: s.path, NodeID: uint64(s.nodeID)}
}

func publicPaint(p boxPaint) Paint {
	border := func(i int) BorderPaint {
		return BorderPaint{Style: BorderStyle(p.borders[i].style), Color: p.borders[i].color}
	}
	return Paint{Background: p.background, Borders: BorderPaints{Top: border(0), Right: border(1), Bottom: border(2), Left: border(3)}}
}
