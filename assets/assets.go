package assets

import _ "embed"

//go:embed ScheherazadeNew-Regular.ttf
var Font []byte

//go:embed OFL.txt
var FontLicense string

const FontFileName = "ScheherazadeNew-Regular.ttf"
const FontFamily = "Scheherazade New"
