// Copyright ©2017 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stl implement STL encoding and decoding and basic STL triangle handling.
package stl

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Vector is a 3D vector.
type Vector struct{ X, Y, Z float64 }

func (v Vector) sub(b Vector) Vector {
	v.X -= b.X
	v.Y -= b.Y
	v.Z -= b.Z
	return v
}

func (v Vector) cross(w Vector) Vector {
	return Vector{
		X: v.Y*w.Z - v.Z*w.Y,
		Y: v.Z*w.X - v.X*w.Z,
		Z: v.X*w.Y - v.Y*w.X,
	}
}

func (v Vector) length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

func (v Vector) scale(f float64) Vector {
	v.X *= f
	v.Y *= f
	v.Z *= f
	return v
}

// Triangle is an STL triangle.
type Triangle struct {
	// Normal holds the stored normal
	// value for the triangle.
	Normal Vector

	// Vertex holds the vertices of
	// the triangle.
	Vertex [3]Vector

	// AttrByteCount is the STL
	// Attribute Byte Count value.
	AttrByteCount uint16
}

// FacetNormal returns the calculated facet normal of the triangle. The returned vector
// is guaranteed to be unit length and may disagree with the stored value.
func (t Triangle) FacetNormal() Vector {
	v := t.Vertex[1].sub(t.Vertex[0])
	w := t.Vertex[2].sub(t.Vertex[0])
	n := v.cross(w)
	return n.scale(1 / n.length())
}

// TextDecoder implements ASCII STL decoding.
type TextDecoder struct {
	Name string // Name holds the solid name of the STL file.

	r *bufio.Reader
}

// NewTextDecoder returns a new ASCII STL decoder.
func NewTextDecoder(r io.Reader) (*TextDecoder, error) {
	dec := TextDecoder{r: bufio.NewReader(r)}
	for {
		b, err := dec.r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		b = bytes.TrimLeft(b, " \t")
		if !bytes.HasPrefix(b, []byte("solid ")) {
			return nil, fmt.Errorf(`stl: file does not begin with "solid ": %q`, b)
		}
		dec.Name = string(bytes.TrimSpace(b[len("solid "):]))
		break
	}
	return &dec, nil
}

// Decode returns the next Triangle in the STL stream.
func (dec *TextDecoder) Decode() (Triangle, error) {
	const (
		facet = iota
		outerloop
		vertex1
		vertex2
		vertex3
		endloop
		endfacet
	)
	expect := facet

	var t Triangle
	for {
		b, err := dec.r.ReadBytes('\n')
		if err != nil {
			return Triangle{}, err
		}
		if len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		b = bytes.TrimLeft(b, " \t")
		switch expect {
		case facet:
			if !bytes.HasPrefix(b, []byte("facet normal ")) {
				if bytes.HasPrefix(b, []byte("endsolid ")) {
					name := string(bytes.TrimSpace(b[len("endsolid "):]))
					if name != "" && name != dec.Name {
						return Triangle{}, fmt.Errorf("stl: unexpected endsolid name: %q", name)
					}
					return Triangle{}, io.EOF
				}
				return Triangle{}, fmt.Errorf(`stl: facet does not begin with "facet normal ": %q`, b)
			}
			t.Normal, err = parseVector(b[len("facet normal "):])
			if err != nil {
				return Triangle{}, err
			}
		case outerloop:
			if !bytes.Equal(bytes.TrimSpace(b), []byte("outer loop")) {
				return Triangle{}, fmt.Errorf(`stl: facet does not contain "outer loop": %q`, b)
			}
		case vertex1, vertex2, vertex3:
			if !bytes.HasPrefix(b, []byte("vertex ")) {
				return Triangle{}, fmt.Errorf(`stl: facet does not contain expected "vertex ": %q`, b)
			}
			t.Vertex[expect-vertex1], err = parseVector(b[len("vertex "):])
			if err != nil {
				return Triangle{}, err
			}
		case endloop:
			if !bytes.Equal(bytes.TrimSpace(b), []byte("endloop")) {
				return Triangle{}, fmt.Errorf(`stl: facet does not contain "endloop": %q`, b)
			}
		case endfacet:
			if !bytes.Equal(bytes.TrimSpace(b), []byte("endfacet")) {
				return Triangle{}, fmt.Errorf(`stl: facet does not contain "endfacet": %q`, b)
			}
			return t, nil
		default:
			panic(fmt.Sprintf("stl: unexpected state: %d", expect))
		}
		expect++
	}
}

func parseVector(b []byte) (Vector, error) {
	var v Vector
	var err error
	s := strings.Fields(string(b))
	if len(s) != 3 {
		return Vector{}, fmt.Errorf("stl: invalid vector text: %q", b)
	}
	v.X, err = strconv.ParseFloat(s[0], 64)
	if err != nil {
		goto end
	}
	v.Y, err = strconv.ParseFloat(s[1], 64)
	if err != nil {
		goto end
	}
	v.Z, err = strconv.ParseFloat(s[2], 64)
	if err != nil {
		goto end
	}
end:
	return v, err
}

// TextEncoder implements text STL encoding.
type TextEncoder struct {
	w io.Writer

	name string

	indent    string
	indentLen int

	err error
}

// NewTextEncoder returns a new text STL encoder. Name and indent must be ASCII encoded.
// The user must call Close after use to ensure a valid STL encoding.
func NewTextEncoder(w io.Writer, name, indent string) (*TextEncoder, error) {
	_, err := fmt.Fprintf(w, "solid %s\n", name)
	if err != nil {
		return nil, err
	}
	enc := TextEncoder{
		w:         w,
		name:      name,
		indent:    strings.Repeat(indent, 3),
		indentLen: len(indent),
	}
	return &enc, nil
}

// Encode encodes t into the STL stream.
func (enc *TextEncoder) Encode(t Triangle) error {
	enc.printIndented(1, "facet normal %g %g %g\n", t.Normal.X, t.Normal.Y, t.Normal.Z)
	enc.printIndented(2, "outer loop\n")
	enc.printIndented(3, "vertex %g %g %g\n", t.Vertex[0].X, t.Vertex[0].Y, t.Vertex[0].Z)
	enc.printIndented(3, "vertex %g %g %g\n", t.Vertex[1].X, t.Vertex[1].Y, t.Vertex[1].Z)
	enc.printIndented(3, "vertex %g %g %g\n", t.Vertex[2].X, t.Vertex[2].Y, t.Vertex[2].Z)
	enc.printIndented(2, "endloop\n")
	enc.printIndented(1, "endfacet\n")
	err := enc.err
	enc.err = nil
	return err
}

func (enc *TextEncoder) printIndented(n int, format string, args ...interface{}) {
	if enc.err != nil {
		return
	}
	_, enc.err = fmt.Fprint(enc.w, enc.indent[:n*enc.indentLen])
	if enc.err != nil {
		return
	}
	_, enc.err = fmt.Fprintf(enc.w, format, args...)
}

// Close closes the encoder.
func (enc *TextEncoder) Close() error {
	_, err := fmt.Fprintf(enc.w, "endsolid %s\n", enc.name)
	return err
}

// BinaryDecoder implements binary STL decoding.
type BinaryDecoder struct {
	// Header holds the STL header data. The data
	// may be zero terminated depending on the
	// input source.
	Header string

	// n holds the file-specified number of triangles.
	// read hold the number of triangles that have been
	// read. They are int64 to for safety on 32-bit
	// systems.
	n, read int64

	r *bufio.Reader
}

// NewBinaryDecoder returns a new binary STL decoder.
func NewBinaryDecoder(r io.Reader) (*BinaryDecoder, error) {
	dec := BinaryDecoder{r: bufio.NewReader(r)}
	var buf [80]byte
	_, err := io.ReadFull(dec.r, buf[:])
	if err != nil {
		return nil, err
	}
	dec.Header = string(buf[:])
	_, err = io.ReadFull(dec.r, buf[:4])
	if err != nil {
		return nil, err
	}
	dec.n = int64(binary.LittleEndian.Uint32(buf[:4]))
	return &dec, nil
}

// NumTriangles returns the number of triangles encoded in the STL stream.
func (dec *BinaryDecoder) NumTriangles() int { return int(dec.n) }

// Decode returns the next Triangle in the STL stream.
func (dec *BinaryDecoder) Decode() (Triangle, error) {
	if dec.n == dec.read {
		return Triangle{}, io.EOF
	}
	var b [50]byte
	_, err := io.ReadFull(dec.r, b[:])
	if err != nil {
		return Triangle{}, err
	}
	t := Triangle{
		Normal: getVector(b[:12]),
		Vertex: [3]Vector{
			getVector(b[12:24]),
			getVector(b[24:36]),
			getVector(b[36:48]),
		},
		AttrByteCount: binary.LittleEndian.Uint16(b[48:]),
	}
	dec.read++
	return t, nil
}

func getVector(b []byte) Vector {
	_ = b[:12]
	return Vector{
		X: float64(math.Float32frombits(binary.LittleEndian.Uint32(b[:4]))),
		Y: float64(math.Float32frombits(binary.LittleEndian.Uint32(b[4:8]))),
		Z: float64(math.Float32frombits(binary.LittleEndian.Uint32(b[8:12]))),
	}
}

// BinaryEncoder implements binary STL encoding.
type BinaryEncoder struct {
	w io.Writer

	// n holds the specified number of triangles.
	// written hold the number of triangles that have
	// been written. They are int64 to for safety on
	// 32-bit systems.
	n, written int64
}

// NewBinaryEncoder returns a new binary STL encoder. The value in header is written
// to the STL header data and n is written to the number of triangles. Only n calls
// to Encode will be allowed.
func NewBinaryEncoder(w io.Writer, header string, n uint32) (*BinaryEncoder, error) {
	enc := BinaryEncoder{w: w, n: int64(n)}
	var b [80]byte
	copy(b[:], header)
	_, err := enc.w.Write(b[:])
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(b[:4], n)
	_, err = enc.w.Write(b[:4])
	if err != nil {
		return nil, err
	}
	return &enc, nil
}

// Encode encodes t into the STL stream.
func (enc *BinaryEncoder) Encode(t Triangle) error {
	if enc.n == enc.written {
		return fmt.Errorf("stl: already written %d triangles", enc.n)
	}

	var b [50]byte
	putVector(b[:12], t.Normal)
	putVector(b[12:24], t.Vertex[0])
	putVector(b[24:36], t.Vertex[1])
	putVector(b[36:48], t.Vertex[2])
	binary.LittleEndian.PutUint16(b[48:], t.AttrByteCount)

	_, err := enc.w.Write(b[:])
	enc.written++
	return err
}

func putVector(b []byte, v Vector) {
	_ = b[:12]
	binary.LittleEndian.PutUint32(b[:4], math.Float32bits(float32(v.X)))
	binary.LittleEndian.PutUint32(b[4:8], math.Float32bits(float32(v.Y)))
	binary.LittleEndian.PutUint32(b[8:12], math.Float32bits(float32(v.Z)))
}
