package rom

import "fmt"

// Kind classifies a free region.
type Kind int

const (
	Interior  Kind = iota // filler run inside the original image; may be blank data
	Tail                  // filler run reaching the end of the original image
	Expansion             // bytes added beyond the original image
)

func (k Kind) String() string {
	switch k {
	case Interior:
		return "interior"
	case Tail:
		return "tail"
	case Expansion:
		return "expansion"
	}
	return "unknown"
}

// Region is a half-open byte range [Start, End) of free space.
type Region struct {
	Start, End int
	Fill       byte
	Kind       Kind
}

// Len returns the number of bytes in r.
func (r Region) Len() int { return r.End - r.Start }

// Scan finds runs of 0x00 or 0xFF after the header, plus the expansion from len(img) to size.
// Interior runs shorter than minLen are dropped; a tail is always kept. Starts round up to even.
func Scan(img []byte, size, minLen int) ([]Region, error) {
	if len(img) < HeaderSize {
		return nil, fmt.Errorf("image is %d bytes, shorter than the %d-byte header", len(img), HeaderSize)
	}
	var out []Region
	for i := HeaderSize; i < len(img); {
		b := img[i]
		if b != 0x00 && b != 0xFF {
			i++
			continue
		}
		j := i
		for j < len(img) && img[j] == b {
			j++
		}
		r := Region{Start: (i + 1) &^ 1, End: j, Fill: b, Kind: Interior}
		if j == len(img) {
			r.Kind = Tail
		}
		if r.Len() > 0 && (r.Kind == Tail || r.Len() >= minLen) {
			out = append(out, r)
		}
		i = j
	}
	if size > len(img) {
		out = append(out, Region{Start: len(img), End: size, Fill: FillByte, Kind: Expansion})
	}
	return out, nil
}

// Usable returns the tail and expansion merged into one region, or false if there is neither.
func Usable(regions []Region) (Region, bool) {
	var u Region
	found := false
	for _, r := range regions {
		if r.Kind == Interior {
			continue
		}
		if !found {
			u, found = r, true
			continue
		}
		u.End = r.End // tail and expansion are adjacent by construction
		u.Kind = Expansion
	}
	return u, found
}
