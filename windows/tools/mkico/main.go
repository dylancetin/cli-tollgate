// Command mkico writes a multi-size .ico from PNG files (used for the
// installer icon): mkico out.ico in-256.png in-48.png …
package main

import (
	"image"
	"image/png"
	"log"
	"os"

	"github.com/dylancetin/cli-tollgate/windows/internal/ico"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: mkico out.ico in.png…")
	}
	var imgs []image.Image
	for _, path := range os.Args[2:] {
		f, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		imgs = append(imgs, img)
	}
	data, err := ico.Encode(imgs)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(os.Args[1], data, 0o644); err != nil {
		log.Fatal(err)
	}
}
