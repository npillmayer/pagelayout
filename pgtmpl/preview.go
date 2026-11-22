package main

import (
	"bytes"
	_ "embed"
	"fmt"
	htmltemplate "html/template"
	"image/color"
	"math"
	"net/http"
	"strconv"

	"github.com/npillmayer/pagelayout/template"
)

//go:embed preview.html
var previewHTML string

type previewFlow struct {
	Name, Color string
	Count       int
}

type previewBorder struct {
	Points string
	Color  color.NRGBA
}

type previewFrame struct {
	template.FrameInfo
	Color   string
	Borders []previewBorder
}

type previewPage struct {
	Collection, ID            string
	IDs                       []string
	Index, Number, Prev, Next int
	Page                      template.PageInfo
	Frames                    []previewFrame
	Flows                     []previewFlow
}

// newPreviewHandler snapshots only compiled values. All pages are rendered
// before returning; requests never read the collection or run layout again.
func newPreviewHandler(catalog *template.Catalog, name string) (http.Handler, error) {
	ids := catalog.IDs()
	if len(ids) == 0 {
		return nil, fmt.Errorf("empty template catalog")
	}
	view, err := htmltemplate.New("preview").Funcs(htmltemplate.FuncMap{
		"number": svgNumber,
		"color":  svgColor,
		"flowStyle": func(color string) htmltemplate.CSS {
			// Colors come exclusively from previewFlowColor, never source CSS.
			return htmltemplate.CSS("--flow-color: " + color)
		},
	}).Parse(previewHTML)
	if err != nil {
		return nil, err
	}
	colors := make(map[string]string)
	pages := make([][]byte, len(ids))
	for i, id := range ids {
		compiled, _ := catalog.Lookup(id)
		page := previewPage{Collection: name, ID: id, IDs: ids, Index: i, Number: i + 1,
			Prev: i - 1, Next: i + 1, Page: compiled.Page()}
		flowIndices := make(map[string]int)
		for _, frame := range compiled.Frames() {
			color, ok := colors[frame.Flow]
			if !ok {
				color = previewFlowColor(len(colors))
				colors[frame.Flow] = color
			}
			index, ok := flowIndices[frame.Flow]
			if !ok {
				index = len(page.Flows)
				flowIndices[frame.Flow] = index
				page.Flows = append(page.Flows, previewFlow{Name: frame.Flow, Color: color})
			}
			page.Flows[index].Count++
			page.Frames = append(page.Frames, previewFrame{FrameInfo: frame, Color: color, Borders: frameBorders(frame)})
		}
		var output bytes.Buffer
		if err := view.Execute(&output, page); err != nil {
			return nil, err
		}
		pages[i] = output.Bytes()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		index := 0
		if value := r.URL.Query().Get("page"); value != "" {
			var err error
			index, err = strconv.Atoi(value)
			if err != nil || index < 0 || index >= len(pages) {
				http.Error(w, "unknown template", http.StatusNotFound)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("Content-Length", strconv.Itoa(len(pages[index])))
		if r.Method == http.MethodGet {
			w.Write(pages[index])
		}
	}), nil
}

func svgNumber(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', -1, 32)
}

func svgColor(c color.NRGBA) string {
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", c.R, c.G, c.B, strconv.FormatFloat(float64(c.A)/255, 'f', -1, 64))
}

func previewFlowColor(index int) string {
	palette := [...]string{"#2563b5", "#b45309", "#7c3cad", "#087f70", "#ba3556", "#60751e", "#5260aa", "#9b4933"}
	if index < len(palette) {
		return palette[index]
	}
	// Continue with distinct hues instead of repeating a fixed palette.
	return fmt.Sprintf("hsl(%.6f, 62%%, 38%%)", math.Mod(float64(index)*137.507764, 360))
}

// Paint the four border sides as trapezoids meeting at their corners. Their
// widths and colors are already resolved by the catalog, including hidden sides.
func frameBorders(frame template.FrameInfo) []previewBorder {
	r, b, p := frame.BorderBox, frame.Border, frame.Paint.Borders
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	x2, y2, x3, y3 := x0+b.Left, y0+b.Top, x1-b.Right, y1-b.Bottom
	var borders []previewBorder
	for _, side := range []struct {
		width  float32
		paint  template.BorderPaint
		points [8]float32
	}{
		{b.Top, p.Top, [8]float32{x0, y0, x1, y0, x3, y2, x2, y2}},
		{b.Right, p.Right, [8]float32{x1, y0, x1, y1, x3, y3, x3, y2}},
		{b.Bottom, p.Bottom, [8]float32{x1, y1, x0, y1, x2, y3, x3, y3}},
		{b.Left, p.Left, [8]float32{x0, y1, x0, y0, x2, y2, x2, y3}},
	} {
		if side.width <= 0 || side.paint.Style != template.BorderSolid {
			continue
		}
		v := side.points
		points := fmt.Sprintf("%s,%s %s,%s %s,%s %s,%s", svgNumber(v[0]), svgNumber(v[1]), svgNumber(v[2]), svgNumber(v[3]), svgNumber(v[4]), svgNumber(v[5]), svgNumber(v[6]), svgNumber(v[7]))
		borders = append(borders, previewBorder{Points: points, Color: side.paint.Color})
	}
	return borders
}
