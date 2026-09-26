// Copyright 2026 The go-images authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package png

import (
	"bytes"
	"image"
	"os"
	"testing"
)

// firstDifference is the first row on which two images differ, or -1 when the
// first rows of both are identical.
func firstDifference(a, b image.Image, rows int) int {
	for y := 0; y < rows; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return y
			}
		}
	}
	return -1
}

// TestThePartialRowsAreTheRealRowsAndAllOfThem.
//
// ⛔ Two claims, and the second is the one that is easy to forget. A row COUNTED
// as arrived must hold what the complete decode holds — otherwise the count tells
// a caller to draw whatever the buffer happened to contain. And the count must be
// the LARGEST that is true, because under-claiming is invisible to the first check
// and shows a caller less of the picture than arrived.
//
// Here the second is exact rather than approximate, which was measured and not
// assumed: a row's bytes are read in one go before anything is stored, so the row
// that ran out is untouched and differs. Over eleven fixtures covering every
// colour model this decoder has — grey 8 and 16 bit, RGB 8 and 16, NRGBA, paletted,
// grey+alpha, RGBA — the first differing row was the count itself, every time.
func TestThePartialRowsAreTheRealRowsAndAllOfThem(t *testing.T) {
	for _, name := range []string{
		"video-001.png",
		"benchGray.png",
		"benchRGB.png",
		"benchNRGBA-gradient.png",
		"benchNRGBA-opaque.png",
		"benchPaletted.png",
		// The colour models the bigger fixtures do not reach. 32x32, so a small
		// cut leaves no complete row at all and says so rather than failing.
		"pngsuite/basn0g16.png",
		"pngsuite/basn2c16.png",
		"pngsuite/basn4a08.png",
		"pngsuite/basn6a08.png",
		"pngsuite/basn3p08.png",
	} {
		t.Run(name, func(t *testing.T) {
			full, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Skipf("no such fixture: %v", err)
			}
			whole, err := Decode(bytes.NewReader(full))
			if err != nil {
				t.Fatalf("the complete fixture does not decode, so nothing below "+
					"means anything: %v", err)
			}
			height := whole.Bounds().Dy()

			seen := 0
			for _, pct := range []int{20, 40, 60, 80, 95} {
				img, rows, err := DecodePartial(bytes.NewReader(full[:len(full)*pct/100]))
				if err == nil {
					t.Fatalf("%d%% of the file decoded completely, so this cut "+
						"exercises nothing", pct)
				}
				if img == nil {
					continue // Too little for a row yet; a larger cut will say more.
				}
				if rows <= 0 {
					t.Errorf("%d%%: an image came back with %d rows", pct, rows)
					continue
				}
				if img.Bounds() != whole.Bounds() {
					t.Errorf("%d%%: bounds %v, want the full size %v",
						pct, img.Bounds(), whole.Bounds())
				}
				fd := firstDifference(img, whole, height)
				switch {
				case fd >= 0 && fd < rows:
					t.Errorf("%d%%: %d rows were offered and row %d already differs "+
						"from the complete decode", pct, rows, fd)
				case fd > rows:
					t.Errorf("%d%%: %d rows offered but the picture is right up to "+
						"row %d — %d rows that arrived are being withheld",
						pct, rows, fd, fd-rows)
				}
				if rows < seen {
					t.Errorf("%d%%: %d rows, fewer than the %d a smaller cut offered",
						pct, rows, seen)
				}
				seen = rows
			}
			if seen == 0 {
				t.Errorf("no cut of this file ever produced a row, so the fixture " +
					"proves nothing")
			}
		})
	}
}

// TestACompleteImageSaysEveryRow: the ordinary case still has to work, and the
// count is the only thing that distinguishes it from a partial one.
func TestACompleteImageSaysEveryRow(t *testing.T) {
	full, err := os.ReadFile("testdata/video-001.png")
	if err != nil {
		t.Fatal(err)
	}
	img, rows, err := DecodePartial(bytes.NewReader(full))
	if err != nil {
		t.Fatalf("DecodePartial on a whole file: %v", err)
	}
	if img == nil {
		t.Fatal("no image")
	}
	if rows != img.Bounds().Dy() {
		t.Errorf("rows = %d, want every one of %d", rows, img.Bounds().Dy())
	}
}

// TestAnInterlacedImageIsRefusedPartially.
//
// ⛔ An Adam7 pass covers a SUBSET of the whole frame — every eighth pixel of
// every eighth row, then the gaps between them — so "rows from the top" describes
// nothing about what arrived. A caller told 40 rows would draw forty rows of which
// seven pixels in eight are missing.
//
// A nil image AND zero rows, because a caller checks one or the other and only one
// of those mistakes is visible.
func TestAnInterlacedImageIsRefusedPartially(t *testing.T) {
	for _, name := range []string{"benchRGB-interlace.png", "gray-gradient.interlaced.png"} {
		t.Run(name, func(t *testing.T) {
			full, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Skipf("no such fixture: %v", err)
			}
			// The premise: the whole file does decode, so a refusal below is about
			// interlacing and not about an unreadable fixture.
			if _, err := Decode(bytes.NewReader(full)); err != nil {
				t.Fatalf("the complete fixture does not decode: %v", err)
			}
			for _, pct := range []int{20, 40, 60, 80} {
				img, rows, err := DecodePartial(bytes.NewReader(full[:len(full)*pct/100]))
				if err == nil {
					t.Fatalf("%d%% decoded completely", pct)
				}
				if img != nil {
					t.Errorf("%d%%: an image was offered for an interlaced file", pct)
				}
				if rows != 0 {
					t.Errorf("%d%%: rows = %d, want 0", pct, rows)
				}
			}
		})
	}
}

// TestNothingIsOfferedBeforeTheFirstRow: a cut inside the header has a size and no
// picture, and offering the allocated buffer would offer a blank rectangle as
// though it were content.
func TestNothingIsOfferedBeforeTheFirstRow(t *testing.T) {
	full, err := os.ReadFile("testdata/video-001.png")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, 8, 40, 60} {
		if n > len(full) {
			break
		}
		img, rows, err := DecodePartial(bytes.NewReader(full[:n]))
		if err == nil {
			t.Errorf("%d bytes decoded completely", n)
		}
		if img != nil || rows != 0 {
			t.Errorf("%d bytes offered an image with %d rows", n, rows)
		}
	}
}
