module github.com/npillmayer/pagelayout

go 1.25.6

require (
	github.com/npillmayer/css-box-layout v0.0.0
	github.com/npillmayer/w3c-dom v0.0.0
	github.com/thatisuday/commando v1.0.4
	golang.org/x/net v0.49.0
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/npillmayer/schuko v0.2.0-alpha.3 // indirect
	github.com/npillmayer/tree v0.0.0-20260127205735-9aa4bb66c981 // indirect
	github.com/thatisuday/clapper v1.0.10 // indirect
	golang.org/x/image v0.23.0 // indirect
)

// The styling integration uses the sibling development modules, including DOM-01.
replace github.com/npillmayer/w3c-dom => ../w3c-dom

replace github.com/npillmayer/css-box-layout => ../css-box-layout

replace github.com/npillmayer/tree => ../tree

replace github.com/npillmayer/schuko => ../schuko
