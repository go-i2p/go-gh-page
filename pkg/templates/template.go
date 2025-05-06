package templates

import _ "embed"

//go:embed main.html
var MainTemplate string

//go:embed doc.html
var DocTemplate string

//go:embed style.css
var StyleTemplate string
