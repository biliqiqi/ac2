package webterm

import _ "embed"

//go:embed node_modules/@xterm/xterm/css/xterm.css
var xtermCSS []byte

//go:embed node_modules/@xterm/xterm/lib/xterm.js
var xtermJS []byte

//go:embed node_modules/@xterm/addon-fit/lib/addon-fit.js
var addonFitJS []byte
