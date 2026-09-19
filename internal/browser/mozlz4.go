package browser

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// mozLz4Magic is the header Firefox puts on the files it compresses itself.
// It is not a standard LZ4 frame: the payload is a bare LZ4 block, with the
// decompressed length in front of it rather than the framing LZ4 normally
// carries.
var mozLz4Magic = []byte("mozLz40\x00")

// maxDecompressed bounds what will be unpacked. A session store is a few
// hundred kilobytes; this leaves room for a very busy browser and refuses to
// turn a corrupt length field into an allocation the size of memory.
const maxDecompressed = 64 << 20

// decodeMozLz4 unpacks one of Firefox's compressed files.
//
// It is written out here rather than taken from a library because the format
// is small, the input is a file this tool did not write, and a decompressor
// is exactly the kind of code that should not be trusted to be careful on our
// behalf. Every read is bounds-checked; a truncated or malformed file is an
// error, never a short answer that looks like a real one.
func decodeMozLz4(raw []byte) ([]byte, error) {
	header := len(mozLz4Magic) + 4
	if len(raw) < header {
		return nil, errors.New("too short to be a Firefox compressed file")
	}
	if string(raw[:len(mozLz4Magic)]) != string(mozLz4Magic) {
		return nil, fmt.Errorf("not a Firefox compressed file (header %q)",
			raw[:len(mozLz4Magic)])
	}
	size := binary.LittleEndian.Uint32(raw[len(mozLz4Magic):header])
	if size > maxDecompressed {
		return nil, fmt.Errorf("claims to unpack to %d bytes, which is more than this reads", size)
	}
	return decodeLZ4Block(raw[header:], int(size))
}

// decodeLZ4Block unpacks a bare LZ4 block.
//
// The format is a sequence of sequences. Each begins with a token byte: the
// high nibble counts literal bytes to copy out, the low nibble a match to
// repeat from earlier in the output. A nibble of 15 means "and more", encoded
// as bytes that add up until one of them is not 255.
//
// The match may overlap what it is producing — that is how LZ4 encodes a run
// — so it is copied one byte at a time rather than with copy(), which would
// read ahead of what has been written.
func decodeLZ4Block(src []byte, size int) ([]byte, error) {
	out := make([]byte, 0, size)
	var i int

	readLength := func(initial int) (int, error) {
		length := initial
		if initial != 15 {
			return length, nil
		}
		for {
			if i >= len(src) {
				return 0, errors.New("the block ends in the middle of a length")
			}
			b := src[i]
			i++
			length += int(b)
			if length > size {
				return 0, errors.New("a length runs past the end of the output")
			}
			if b != 255 {
				return length, nil
			}
		}
	}

	for i < len(src) {
		token := src[i]
		i++

		literals, err := readLength(int(token >> 4))
		if err != nil {
			return nil, err
		}
		if i+literals > len(src) {
			return nil, errors.New("the block ends in the middle of its literals")
		}
		out = append(out, src[i:i+literals]...)
		i += literals

		// The last sequence is literals alone: a block ends there, with no
		// match after it.
		if i >= len(src) {
			break
		}
		if i+2 > len(src) {
			return nil, errors.New("the block ends in the middle of a match offset")
		}
		offset := int(binary.LittleEndian.Uint16(src[i:]))
		i += 2
		if offset == 0 || offset > len(out) {
			return nil, fmt.Errorf("a match points %d bytes back, outside what has been read", offset)
		}
		matchLen, err := readLength(int(token & 0x0F))
		if err != nil {
			return nil, err
		}
		matchLen += 4 // the minimum match, which the format leaves implicit

		start := len(out) - offset
		for n := 0; n < matchLen; n++ {
			if len(out) >= size {
				return nil, errors.New("the block produces more than it said it would")
			}
			out = append(out, out[start+n])
		}
	}

	if len(out) != size {
		return nil, fmt.Errorf("unpacked %d bytes, not the %d it said", len(out), size)
	}
	return out, nil
}
