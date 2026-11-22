package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/npillmayer/pagelayout/template"
	"golang.org/x/net/html"
)

func TestPreviewCatalogGeometryAndNavigation(t *testing.T) {
	catalog, err := loadCollection("../template/testdata/templates/sketch.html", "")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newPreviewHandler(catalog, "sketch.html")
	if err != nil {
		t.Fatal(err)
	}
	colors := map[string]string{}
	for i, id := range catalog.IDs() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/?page="+strconv.Itoa(i), nil))
		if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("unexpected response: %v", response.Result())
		}
		doc := parsePreview(t, response.Body.String())
		pages := previewNodes(doc, "svg", "page")
		if len(pages) != 1 {
			t.Fatalf("got %d visible pages, want one", len(pages))
		}
		compiled, _ := catalog.Lookup(id)
		page := compiled.Page()
		if previewAttr(pages[0], "viewBox") != "0 0 "+svgNumber(page.Width)+" "+svgNumber(page.Height) {
			t.Fatalf("wrong page viewport: %v", pages[0].Attr)
		}
		frames := previewNodes(pages[0], "g", "frame")
		if len(frames) != len(compiled.Frames()) {
			t.Fatalf("got %d frames", len(frames))
		}
		for j, frame := range compiled.Frames() {
			if previewAttr(frames[j], "data-flow") != frame.Flow || previewAttr(frames[j], "data-frame-id") != strconv.Itoa(int(frame.ID)) {
				t.Fatal("lost frame identity or flow")
			}
			outlines := previewNodes(frames[j], "rect", "frame-outline")
			if len(outlines) != 1 {
				t.Fatal("missing default outline")
			}
			assertPreviewRect(t, outlines[0], frame.BorderBox)
			style := previewAttr(frames[j], "style")
			if prev, ok := colors[frame.Flow]; ok && prev != style {
				t.Fatal("flow color changed between frames/templates")
			}
			for flow, other := range colors {
				if flow != frame.Flow && other == style {
					t.Fatal("different flows share a color")
				}
			}
			colors[frame.Flow] = style
		}
		legend := previewNodes(doc, "span", "frame-ids")
		wantLegend := [][]string{{"#1", "#2, #3"}, {"#1", "#2"}}[i]
		if len(legend) != len(wantLegend) {
			t.Fatalf("got %d legend entries, want %d", len(legend), len(wantLegend))
		}
		for j, want := range wantLegend {
			if legend[j].FirstChild == nil || legend[j].FirstChild.Data != want {
				t.Fatalf("legend entry %d must show frame numbers %q", j, want)
			}
		}
		links := previewNodes(doc, "a", "button")
		if len(links) != 1 || previewAttr(links[0], "href") != "?page="+strconv.Itoa(1-i) {
			t.Fatal("incorrect previous/next navigation")
		}
		styles := previewNodes(doc, "style", "")
		if previewAttr(styles[0], "id") != "frame-defaults" || !strings.Contains(response.Body.String(), `id="show-frames" checked`) {
			t.Fatal("default border CSS must be prepended and enabled")
		}
	}
}

func TestPreviewPaintAndEscaping(t *testing.T) {
	const input = `<body><template id="&lt;/title&gt;&lt;script&gt;alert(1)&lt;/script&gt;" data-type="page">
<div style="width:300px;height:200px;background:#ffeecc">
<div style="position:fixed;left:10px;top:20px;width:100px;height:80px;flow-from:Main;padding:3px 4px 5px 6px;border:2px solid #ff0000;border-left-width:4px;background:#00ff00"></div>
</div></template></body>`
	catalog, err := template.Load(strings.NewReader(input), template.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newPreviewHandler(catalog, `<script>alert(2)</script>.html`)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	doc := parsePreview(t, response.Body.String())
	if len(previewNodes(doc, "script", "")) != 0 || strings.Contains(response.Body.String(), "ZgotmplZ") {
		t.Fatal("source text escaped incorrectly")
	}
	if !strings.Contains(response.Body.String(), "&lt;script&gt;alert(2)&lt;/script&gt;.html") {
		t.Fatal("collection name not escaped")
	}
	compiled, _ := catalog.Lookup(catalog.IDs()[0])
	frame := compiled.Frames()[0]
	background := previewNodes(doc, "rect", "frame-background")[0]
	assertPreviewRect(t, background, frame.BorderBox)
	if previewAttr(background, "fill") != "rgba(0,255,0,1)" {
		t.Fatal("lost frame paint")
	}
	page := previewNodes(doc, "rect", "page-background")[0]
	if previewAttr(page, "fill") != "rgba(255,238,204,1)" {
		t.Fatal("lost page paint")
	}
	borders := previewNodes(doc, "polygon", "frame-border")
	if len(borders) != 4 || previewAttr(borders[0], "points") != "10,20 126,20 124,22 14,22" {
		t.Fatalf("incorrect asymmetric border geometry: %v", borders)
	}
	if !strings.Contains(response.Body.String(), "Content: x=20, y=25, width=100, height=80 CSS px") {
		t.Fatal("content rectangle lost its border/padding offset")
	}
}

func TestPreviewSnapshotAndHTTP(t *testing.T) {
	name := filepath.Join(t.TempDir(), "pages.html")
	if err := os.WriteFile(name, []byte(`<body><template id="empty" data-type="page"><div style="width:200px;height:100px"></div></template></body>`), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := loadCollection(name, "")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newPreviewHandler(catalog, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, url string
		status      int
	}{
		{"GET", "/", 200}, {"HEAD", "/", 200}, {"POST", "/", 405},
		{"GET", "/?page=-1", 404}, {"GET", "/?page=1", 404}, {"GET", "/?page=bad", 404},
		{"GET", "/pages.html", 404},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(tc.method, tc.url, nil))
		if response.Code != tc.status {
			t.Fatalf("%s %s: got %d", tc.method, tc.url, response.Code)
		}
		if tc.method == "HEAD" && response.Body.Len() != 0 {
			t.Fatal("HEAD returned a body")
		}
		if tc.method == "GET" && tc.status == 200 {
			doc := parsePreview(t, response.Body.String())
			if len(previewNodes(doc, "g", "frame")) != 0 || len(previewNodes(doc, "a", "button")) != 0 {
				t.Fatal("single empty page has frames or previous/next links")
			}
		}
	}
	if handler, err := newPreviewHandler(nil, "empty"); err == nil || handler != nil {
		t.Fatal("empty catalog published a preview")
	}
}

func TestServeRejectsCollectionBeforeListening(t *testing.T) {
	var report bytes.Buffer
	status := runServe(context.Background(), "../template/testdata/templates/invalid-overlap.html", "invalid-listen-address", "", &report)
	if status != 1 || !strings.Contains(report.String(), "geometry.overlap") || strings.Contains(report.String(), "pgtmpl: serve") {
		t.Fatalf("invalid collection must fail before listening: exit %d, %s", status, report.String())
	}
}

func TestServeLifecycle(t *testing.T) {
	cssPath := filepath.Join(t.TempDir(), "custom.css")
	if err := os.WriteFile(cssPath, []byte(`.page {background:#abcdef}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output := make(serveTestOutput, 8)
	result := make(chan int, 1)
	go func() {
		result <- runServe(ctx, "../template/testdata/templates/sketch.html", "127.0.0.1:0", cssPath, output)
	}()
	select {
	case message := <-output:
		var url string
		for _, field := range strings.Fields(message) {
			if strings.HasPrefix(field, "http://") {
				url = field
			}
		}
		if url == "" {
			t.Fatalf("missing server URL: %s", message)
		}
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get(url + "?page=1")
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || !bytes.Contains(body, []byte("<h1>chapter-opening</h1>")) {
			t.Fatalf("server returned unexpected preview: status=%d err=%v", response.StatusCode, err)
		}
		if !bytes.Contains(body, []byte(`fill="rgba(171,205,239,1)"`)) {
			t.Fatal("served preview did not use the override's compiled paint")
		}
	case status := <-result:
		t.Fatalf("server exited before startup: %d", status)
	case <-ctx.Done():
		t.Fatal("server startup timed out")
	}
	cancel()
	select {
	case status := <-result:
		if status != 0 {
			t.Fatalf("shutdown exited with status %d", status)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("server shutdown timed out")
	}
}

type serveTestOutput chan string

func (out serveTestOutput) Write(p []byte) (int, error) {
	out <- string(p)
	return len(p), nil
}

func parsePreview(t *testing.T, text string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func previewNodes(root *html.Node, tag, class string) []*html.Node {
	var nodes []*html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tag && (class == "" || previewAttr(n, "class") == class) {
			nodes = append(nodes, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(root)
	return nodes
}

func previewAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func assertPreviewRect(t *testing.T, n *html.Node, rect template.Rect) {
	t.Helper()
	for key, want := range map[string]float32{"x": rect.X, "y": rect.Y, "width": rect.W, "height": rect.H} {
		value, err := strconv.ParseFloat(previewAttr(n, key), 32)
		if err != nil || float32(value) != want {
			t.Fatalf("%s: got %s, want %g", key, previewAttr(n, key), want)
		}
	}
}
