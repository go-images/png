# png

> **A fork of Go's `image/png`**, under the same BSD-3-Clause licence.
> The import path is `github.com/go-images/png`. Every change is stated in
> [NOTICE](NOTICE), and Go's own test suite passes here unaltered in
> substance.

**What differs:** `DecodePartial` says what a stream that stopped early did
decode.

```go
img, rows, err := png.DecodePartial(r)
```

The image at its full declared size, the number of pixel rows that are
complete counting from the top, and the error that stopped the decode — `nil`
when everything arrived. Rows past the count hold whatever the image was
allocated with, so a caller draws the first `rows` and nothing else.

`Decode` is all-or-nothing, and for a file still arriving that is the same as
having nothing: it returns a nil image and `not enough pixel data` whatever
fraction is present. `image/jpeg` and `image/gif` answer the same way, so this
is a gap in the shape of every decoder rather than in one of them.

What `DecodePartial` returns for a 150×103 picture:

| bytes given | rows offered | |
|---:|---:|---:|
| 20% | 25 | 24.3% |
| 40% | 48 | 46.6% |
| 60% | 66 | 64.1% |
| 80% | 84 | 81.6% |
| 100% | 103 | 100% |

## The count is exact, in both directions

**It never names the row being decoded.** A row's bytes are read in one go
before anything is stored, so the row that ran out is untouched — and claiming
it would hand a caller a band of unfiltered noise.

**And it is the largest count that is true.** Under-claiming is invisible to a
check that only verifies the rows it offers, and it shows a caller less of the
picture than arrived. Measured over eleven fixtures covering every colour model
this decoder has — grey 8 and 16 bit, RGB 8 and 16, NRGBA, paletted,
grey+alpha, RGBA — the first row that differs from the complete decode **is**
the count, every time. The test asserts that equality rather than a tolerance.

## What is refused partially

An **interlaced** (Adam7) image, with a nil image and zero rows. A pass covers a
subset of the whole frame — every eighth pixel of every eighth row, then the
gaps between them — so "rows from the top" describes nothing about what arrived.
A caller told forty rows would draw forty rows of which seven pixels in eight
are missing.

So is anything that failed **before the first row completed**: there is a size,
but no picture.

## Everything else

Identical to the standard library, including `Encode`. Use it exactly as you
would use `image/png`.
