package webterm

import _ "embed"

//go:embed assets/xterm.css
var xtermCSS []byte

//go:embed assets/xterm.js
var xtermJS []byte

//go:embed assets/addon-fit.js
var addonFitJS []byte
