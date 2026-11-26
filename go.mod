module github.com/npillmayer/pagelayout

go 1.25.6

require (
	github.com/npillmayer/css-box-layout v0.0.0
	github.com/npillmayer/khipu v0.0.0
	github.com/npillmayer/typography v0.0.0
	github.com/npillmayer/uax v0.3.0-alpha1
	github.com/npillmayer/w3c-dom v0.0.0
	github.com/thatisuday/commando v1.0.4
	golang.org/x/net v0.49.0
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/jolestar/go-commons-pool v2.0.0+incompatible // indirect
	github.com/npillmayer/cords v0.1.2-alpha.3 // indirect
	github.com/npillmayer/opentype v0.0.0 // indirect
	github.com/npillmayer/schuko v0.2.0-alpha.3.0.20211209143531-2d524c4964ff // indirect
	github.com/npillmayer/tree v0.0.0-20260127205735-9aa4bb66c981 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	github.com/thatisuday/clapper v1.0.10 // indirect
	golang.org/x/image v0.34.0 // indirect
	golang.org/x/text v0.33.0 // indirect
)

// Local replacements connect the sibling typesetting modules.
replace github.com/npillmayer/w3c-dom => ../w3c-dom

replace github.com/npillmayer/css-box-layout => ../css-box-layout

replace github.com/npillmayer/tree => ../tree

replace github.com/npillmayer/schuko => ../schuko

// Optional typography preparation uses the sibling development modules.
// OpenType supports explicit test adapters, not the bridge's production imports.
replace github.com/npillmayer/typography => ../typography

replace github.com/npillmayer/khipu => ../khipu

replace github.com/npillmayer/cords => ../cords

replace github.com/npillmayer/uax => ../uax

replace github.com/npillmayer/opentype => ../opentype
