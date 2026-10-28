package boxart

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPrepare(t *testing.T) {
	small := pngBytes(t, 200, 150)
	out, ext, err := Prepare(small)
	if err != nil || ext != "png" || !bytes.Equal(out, small) {
		t.Errorf("small png should pass through unchanged: ext=%q err=%v same=%v", ext, err, bytes.Equal(out, small))
	}

	j := jpegBytes(t, 300, 300)
	out, ext, err = Prepare(j)
	if err != nil || ext != "jpg" || !bytes.Equal(out, j) {
		t.Errorf("small jpeg: ext=%q err=%v", ext, err)
	}

	webp := append([]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), make([]byte, 20)...)
	out, ext, err = Prepare(webp)
	if err != nil || ext != "webp" || !bytes.Equal(out, webp) {
		t.Errorf("webp: ext=%q err=%v", ext, err)
	}

	// Larger than MaxSide: scaled down (aspect kept) and re-encoded as PNG.
	big := jpegBytes(t, 1800, 900)
	out, ext, err = Prepare(big)
	if err != nil || ext != "png" {
		t.Fatalf("big jpeg: ext=%q err=%v", ext, err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != MaxSide || cfg.Height != MaxSide/2 {
		t.Errorf("big jpeg scaled to %dx%d (err %v), want %dx%d", cfg.Width, cfg.Height, err, MaxSide, MaxSide/2)
	}

	var g bytes.Buffer
	_ = gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 10, 10), color.Palette{color.Black, color.White}), nil)
	if _, ext, err = Prepare(g.Bytes()); err != nil || ext != "gif" {
		t.Errorf("gif: ext=%q err=%v", ext, err)
	}

	for name, bad := range map[string][]byte{
		"empty": nil, "text": []byte("<html>not found</html>"), "truncated png": small[:20],
	} {
		if _, _, err := Prepare(bad); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestPrepareRejectsHugeDimensions(t *testing.T) {
	// A PNG header claiming 20000x20000 must be refused without decoding it.
	var b bytes.Buffer
	img := image.NewGray(image.Rect(0, 0, 20000, 20000))
	_ = png.Encode(&b, img)
	if _, _, err := Prepare(b.Bytes()); err == nil {
		t.Error("want error for oversized image")
	}
}

// fixture builds a fake device: a catalogue root, content configs and an
// optional assign.json.
type fixture struct {
	root, cat, content string
	fetched            []string
	data               []byte
	fetchErr           error
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{root: root, cat: filepath.Join(root, "info", "catalogue"), content: filepath.Join(root, "content"), data: pngBytes(t, 100, 100)}
	for _, d := range []string{f.cat, f.content, filepath.Join(root, "info", "manifest")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *fixture) write(t *testing.T, rel, content string) {
	t.Helper()
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) saver() *Saver {
	return &Saver{
		CatalogueDirs: []string{filepath.Join(f.root, "nope"), f.cat},
		ContentDir:    f.content,
		Fetch: func(u string) ([]byte, error) {
			f.fetched = append(f.fetched, u)
			return f.data, f.fetchErr
		},
	}
}

func TestSaveWritesToAssignedCatalogue(t *testing.T) {
	f := newFixture(t)
	f.write(t, "content/SFC/core.cfg", "snes9x_libretro.so\nNintendo SNES - SFC\nNintendo SNES - SFC\n0\nmu-snes9x")
	f.write(t, "info/catalogue/Nintendo SNES - SFC/box/.keep", "")
	res, err := f.saver().Save("SFC", "Super Mario World (U).smc", "http://x/cover")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(f.cat, "Nintendo SNES - SFC", "box", "Super Mario World (U).png")
	if res.Path != want {
		t.Fatalf("path %q, want %q (skip %q)", res.Path, want, res.Skip)
	}
	got, err := os.ReadFile(want)
	if err != nil || !bytes.Equal(got, f.data) {
		t.Errorf("file content wrong: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(want))
	for _, e := range entries {
		if strings.Contains(e.Name(), "tmp") || strings.HasSuffix(e.Name(), ".part") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestSaveNamesFromFileOnDisk(t *testing.T) {
	cases := map[string]string{
		"Mr. Do! (USA).nes":      "Mr. Do! (USA).png",
		"Pokémon – Émeraude.gba": "Pokémon – Émeraude.png",
		"Game.v1.2.gb":           "Game.v1.2.png",
		"Title-123.gba":          "Title-123.png",
		"日本語 (Japan).sfc":        "日本語 (Japan).png",
	}
	for file, want := range cases {
		f := newFixture(t)
		f.write(t, "info/manifest/assign.json", `{"sfc":"Cat"}`)
		f.write(t, "info/catalogue/Cat/box/.keep", "")
		res, err := f.saver().Save("SFC", file, "u")
		if err != nil || filepath.Base(res.Path) != want {
			t.Errorf("%q: path %q err %v, want %q", file, res.Path, err, want)
		}
	}
}

func TestSavePerROMCfgWins(t *testing.T) {
	f := newFixture(t)
	f.write(t, "content/SFC/core.cfg", "c\ns\nFolderCat\n0")
	f.write(t, "content/SFC/Game (U).cfg", "Game (U)\nc\ns\nROMCat\n0")
	res, err := f.saver().Save("SFC", "Game (U).smc", "u")
	if err != nil || !strings.Contains(res.Path, "/ROMCat/box/") {
		t.Fatalf("got %+v err %v", res, err)
	}
}

func TestSaveCreatesBoxDirForAssignedCatalogue(t *testing.T) {
	f := newFixture(t)
	f.write(t, "content/GBA/core.cfg", "c\ns\nGBA Cat\n0")
	res, err := f.saver().Save("GBA", "a.gba", "u")
	if err != nil || res.Path == "" {
		t.Fatalf("got %+v err %v", res, err)
	}
}

func TestSaveAssignFallbackNeedsExistingCatalogueDir(t *testing.T) {
	f := newFixture(t)
	f.write(t, "info/manifest/assign.json", `{"gba":"Nintendo Game Boy Advance"}`)
	res, err := f.saver().Save("GBA", "a.gba", "u")
	if err != nil || res.Path != "" || res.Skip == "" {
		t.Fatalf("missing catalogue dir must skip: %+v err %v", res, err)
	}
	if len(f.fetched) != 0 {
		t.Error("cover fetched for a skipped ROM")
	}
	if _, err := os.Stat(filepath.Join(f.cat, "Nintendo Game Boy Advance")); err == nil {
		t.Error("catalogue folder must not be invented from the fallback table")
	}
	if err := os.MkdirAll(filepath.Join(f.cat, "Nintendo Game Boy Advance"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err = f.saver().Save("GBA", "a.gba", "u")
	if err != nil || res.Path == "" {
		t.Fatalf("existing dir should work: %+v err %v", res, err)
	}
}

func TestSaveSkips(t *testing.T) {
	t.Run("no catalogue for platform", func(t *testing.T) {
		f := newFixture(t)
		res, err := f.saver().Save("_UNSORTED", "a.rom", "u")
		if err != nil || res.Path != "" || res.Skip == "" || len(f.fetched) != 0 {
			t.Errorf("%+v %v fetched=%d", res, err, len(f.fetched))
		}
	})
	t.Run("no catalogue root on this machine", func(t *testing.T) {
		f := newFixture(t)
		s := f.saver()
		s.CatalogueDirs = []string{filepath.Join(f.root, "missing")}
		res, err := s.Save("SFC", "a.smc", "u")
		if err != nil || res.Skip == "" || len(f.fetched) != 0 {
			t.Errorf("%+v %v", res, err)
		}
	})
	t.Run("art already present in any case or format", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, "content/SFC/core.cfg", "c\ns\nCat\n0")
		f.write(t, "info/catalogue/Cat/box/GAME (u).JPG", "keep me")
		res, err := f.saver().Save("SFC", "Game (U).smc", "u")
		if err != nil || res.Path != "" || res.Skip == "" || len(f.fetched) != 0 {
			t.Errorf("%+v %v fetched=%d", res, err, len(f.fetched))
		}
		b, _ := os.ReadFile(filepath.Join(f.cat, "Cat", "box", "GAME (u).JPG"))
		if string(b) != "keep me" {
			t.Error("existing art was modified")
		}
	})
	t.Run("art present in the WxH folder", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, "content/SFC/core.cfg", "c\ns\nCat\n0")
		f.write(t, "info/catalogue/Cat/box/640x480/Game.png", "x")
		res, _ := f.saver().Save("SFC", "Game.smc", "u")
		if res.Path != "" || res.Skip == "" {
			t.Errorf("%+v", res)
		}
	})
	t.Run("hostile names", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, "content/SFC/core.cfg", "c\ns\nCat\n0")
		for _, name := range []string{"", "../evil.smc", "a/b.smc", ".hidden.smc", ".smc"} {
			res, err := f.saver().Save("SFC", name, "u")
			if err != nil || res.Path != "" || res.Skip == "" {
				t.Errorf("%q: %+v %v", name, res, err)
			}
		}
		if res, _ := f.saver().Save("../x", "a.smc", "u"); res.Path != "" {
			t.Error("hostile folder accepted")
		}
	})
	t.Run("empty cover url", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, "content/SFC/core.cfg", "c\ns\nCat\n0")
		res, err := f.saver().Save("SFC", "a.smc", "")
		if err != nil || res.Path != "" || res.Skip == "" || len(f.fetched) != 0 {
			t.Errorf("%+v %v", res, err)
		}
	})
}

func TestSaveFailuresLeaveNothing(t *testing.T) {
	f := newFixture(t)
	f.write(t, "content/SFC/core.cfg", "c\ns\nCat\n0")
	f.fetchErr = errors.New("boom")
	f.data = nil
	if _, err := f.saver().Save("SFC", "a.smc", "u"); err == nil {
		t.Error("fetch failure must be reported")
	}
	f.fetchErr = nil
	f.data = []byte("<html>404</html>")
	if _, err := f.saver().Save("SFC", "a.smc", "u"); err == nil {
		t.Error("non-image body must be reported")
	}
	if entries, _ := os.ReadDir(filepath.Join(f.cat, "Cat", "box")); len(entries) != 0 {
		t.Errorf("files left after failures: %v", entries)
	}
}
