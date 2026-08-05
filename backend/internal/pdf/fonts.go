package pdf

import (
	"embed"
)

//go:embed fonts/DejaVuSans.ttf fonts/DejaVuSans-Bold.ttf
var fontsFS embed.FS

var (
	dejaVuRegular = mustReadFont("fonts/DejaVuSans.ttf")
	dejaVuBold    = mustReadFont("fonts/DejaVuSans-Bold.ttf")
)

func mustReadFont(name string) []byte {
	data, err := fontsFS.ReadFile(name)
	if err != nil {
		panic("pdf: no se pudo embeder la fuente " + name + ": " + err.Error())
	}
	return data
}
