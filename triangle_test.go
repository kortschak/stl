// Copyright ©2017 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stl

import (
	"bytes"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"testing"
)

var facetNormalTests = []struct {
	triangle Triangle
	want     Vector
	tol      float64
}{
	{triangle: Triangle{}, want: Vector{math.NaN(), math.NaN(), math.NaN()}},
	{
		triangle: Triangle{Vertex: [3]Vector{
			{0, 0, 0},
			{1, 0, 0},
			{0, 1, 0},
		}},
		want: Vector{Z: 1}, tol: 1e-14,
	},
	{
		triangle: Triangle{Vertex: [3]Vector{
			{0, 1, 0},
			{1, 0, 0},
			{0, 0, 0},
		}},
		want: Vector{Z: -1}, tol: 1e-14,
	},
	{
		triangle: Triangle{Vertex: [3]Vector{
			{0, 0, 0},
			{1, 1, 0},
			{0, 0, 1},
		}},
		want: Vector{math.Sqrt2 / 2, -math.Sqrt2 / 2, 0}, tol: 1e-14,
	},

	// Examples from a wild STL file.
	{
		triangle: Triangle{Vertex: [3]Vector{
			{1.87881290913, -3.53213620186, -11.3913011551},
			{-2.68665337563, 4.77704572678, -10.5945053101},
			{6.36734676361, 2.7538228035, -9.43354320526},
		}},
		want: Vector{0.165308400989, 0.183746621013, -0.968973815441}, tol: 1e-7,
	},
	{
		triangle: Triangle{Vertex: [3]Vector{
			{1.87881290913, -3.53213620186, -11.3913011551},
			{-5.0690164566, -6.2167840004, -8.72221755981},
			{-2.68665337563, 4.77704572678, -10.5945053101},
		}},
		want: Vector{-0.32688832283, -0.089389257133, -0.940825998783}, tol: 1e-7,
	},
}

func TestFacetNormal(t *testing.T) {
	for _, test := range facetNormalTests {
		got := test.triangle.FacetNormal()
		if !sameVector(got, test.want, test.tol) {
			t.Errorf("unexpected facet normal vector:\ngot:\n\t%+v\nwant:\n\t%+v", got, test.want)
		}
		len := got.length()
		if !math.IsNaN(len) && !sameFloat64(len, 1, test.tol) {
			t.Errorf("unexpected facet normal length: got:%f want:1", len)
		}
	}
}

func TestTextDecoder(t *testing.T) {
	f, err := os.Open(filepath.FromSlash("testdata/low_ascii.stl"))
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()
	dec, err := NewTextDecoder(f)
	if err != nil {
		t.Fatalf("failed to make decoder: %v", err)
	}

	want := `"low"`
	if dec.Name != want {
		t.Errorf(`unexpected solid name: got:%v want:%v`, dec.Name, want)
	}
	for {
		tri, err := dec.Decode()
		if err != nil {
			break
		}
		n := tri.FacetNormal()
		if !sameVector(n, tri.Normal, 1e-6) {
			t.Errorf("unexpected facet normal vector:\ngot:\n\t%+v\nwant:\n\t%+v", n, tri.Normal)
		}
		len := tri.Normal.length()
		if !sameFloat64(len, 1, 1e-6) {
			t.Errorf("unexpected normal length: got:%f want:1", len)
		}
		len = n.length()
		if !sameFloat64(len, 1, 1e-14) {
			t.Errorf("unexpected facet normal length: got:%f want:1", len)
		}
	}
}

func TestTextRoundTrip(t *testing.T) {
	want, err := ioutil.ReadFile(filepath.FromSlash("testdata/low_ascii.stl"))
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	dec, err := NewTextDecoder(bytes.NewReader(want))
	if err != nil {
		t.Fatalf("failed to make decoder: %v", err)
	}
	var buf bytes.Buffer
	enc, err := NewTextEncoder(&buf, dec.Name, "  ")
	if err != nil {
		t.Fatalf("failed to make encoder: %v", err)
	}

	for {
		tri, err := dec.Decode()
		if err != nil {
			break
		}
		err = enc.Encode(tri)
		if err != nil {
			t.Fatalf("could not write triangle: %v", err)
		}
	}

	err = enc.Close()
	if err != nil {
		t.Errorf("unexpected close error: %v", err)
	}

	got := buf.Bytes()
	if !bytes.Equal(got, want) {
		t.Error("roundtrip disagrees")
	}
}

func TestBinaryDecoder(t *testing.T) {
	f, err := os.Open(filepath.FromSlash("testdata/low_binary.stl"))
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()
	dec, err := NewBinaryDecoder(f)
	if err != nil {
		t.Fatalf("failed to make decoder: %v", err)
	}

	want := "MESHMIXER-STL-BINARY-FORMAT----------------------------------------------------\x00"
	if dec.Header != want {
		t.Errorf(`unexpected header: got:%q want:%q`, dec.Header, want)
	}
	for {
		tri, err := dec.Decode()
		if err != nil {
			break
		}
		n := tri.FacetNormal()
		if !sameVector(n, tri.Normal, 1e-6) {
			t.Errorf("unexpected facet normal vector:\ngot:\n\t%+v\nwant:\n\t%+v", n, tri.Normal)
		}
		len := tri.Normal.length()
		if !sameFloat64(len, 1, 1e-6) {
			t.Errorf("unexpected normal length: got:%f want:1", len)
		}
		len = n.length()
		if !sameFloat64(len, 1, 1e-14) {
			t.Errorf("unexpected facet normal length: got:%f want:1", len)
		}
		// Meshmixer apparently uses the Attribute Byte Count.
		if tri.AttrByteCount != 0xffff {
			t.Errorf("unexpected attribute byte count: got:0x%x want:0xffff", tri.AttrByteCount)
		}
	}
}

func TestBinaryRoundTrip(t *testing.T) {
	want, err := ioutil.ReadFile(filepath.FromSlash("testdata/low_binary.stl"))
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	dec, err := NewBinaryDecoder(bytes.NewReader(want))
	if err != nil {
		t.Fatalf("failed to make decoder: %v", err)
	}
	var buf bytes.Buffer
	enc, err := NewBinaryEncoder(&buf, dec.Header, uint32(dec.NumTriangles()))
	if err != nil {
		t.Fatalf("failed to make encoder: %v", err)
	}

	for {
		tri, err := dec.Decode()
		if err != nil {
			break
		}
		err = enc.Encode(tri)
		if err != nil {
			t.Fatalf("could not write triangle: %v", err)
		}
	}

	got := buf.Bytes()
	if !bytes.Equal(got, want) {
		t.Error("roundtrip disagrees")
	}
}

func TestDecoderAgreement(t *testing.T) {
	fa, err := os.Open(filepath.FromSlash("testdata/low_ascii.stl"))
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer fa.Close()
	ascii, err := NewTextDecoder(fa)
	if err != nil {
		t.Fatalf("failed to make text decoder: %v", err)
	}

	fb, err := os.Open(filepath.FromSlash("testdata/low_binary.stl"))
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer fb.Close()
	bin, err := NewBinaryDecoder(fb)
	if err != nil {
		t.Fatalf("failed to make binary decoder: %v", err)
	}

loop:
	for {
		atri, aerr := ascii.Decode()
		btri, berr := bin.Decode()

		if !sameTriangle(atri, btri, 1e-10) {
			t.Errorf("decoders disagree:\nascii:\n\t%+v\nbinary:\n\t%+v", atri, btri)
			break
		}

		switch {
		case aerr != nil && berr != nil:
			break loop
		case aerr != nil, berr != nil:
			t.Errorf("decoders disagree on error state: ascii:%v binary:%v", aerr, berr)
			break loop
		}
	}
}

func sameTriangle(a, b Triangle, tol float64) bool {
	return sameVector(a.Normal, b.Normal, tol) &&
		sameVector(a.Vertex[0], b.Vertex[0], tol) &&
		sameVector(a.Vertex[1], b.Vertex[1], tol) &&
		sameVector(a.Vertex[2], b.Vertex[2], tol)
}

func sameVector(a, b Vector, tol float64) bool {
	return sameFloat64(a.X, b.X, tol) && sameFloat64(a.Y, b.Y, tol) && sameFloat64(a.Z, b.Z, tol)
}

func sameFloat64(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol || (math.IsNaN(a) && math.IsNaN(b))
}
