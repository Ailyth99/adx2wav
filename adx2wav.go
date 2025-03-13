
// ADX DOCUMENT (JAPANESE TEXT)
//https://web.archive.org/web/20020203033404/http://ku-www.ss.titech.ac.jp/~yatsushi/adx.html
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// wavHeader, simple wav header.
type wavHeader struct {
	Riff          [4]byte
	Size          uint32
	Wave          [4]byte
	Fmt           [4]byte
	FmtSize       uint32
	Format        uint16
	Channels      uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	Data          [4]byte
	DataSize      uint32
}

// prevSample, stores previous samples for ADX decoding.
type prevSample struct {
	s1 int
	s2 int
}

// adx2pcm, decodes a single ADX block to PCM.
func adx2pcm(out []int16, adxData []byte, prev *prevSample, coef1, coef2 int) {
	scale := int(binary.BigEndian.Uint16(adxData[0:2])) + 1
	s1, s2 := prev.s1, prev.s2

	adxData = adxData[2:]

	for i := 0; i < 16; i++ {
		d := int(adxData[i] >> 4)
		if d&8 != 0 {
			d -= 16
		}
		s0 := d*scale + ((coef1*s1 + coef2*s2) >> 12)
		s0 = clip(s0, -32768, 32767)
		out[i*2] = int16(s0)
		s2, s1 = s1, s0

		d = int(adxData[i] & 15)
		if d&8 != 0 {
			d -= 16
		}
		s0 = d*scale + ((coef1*s1 + coef2*s2) >> 12)
		s0 = clip(s0, -32768, 32767)
		out[i*2+1] = int16(s0)
		s2, s1 = s1, s0
	}

	prev.s1, prev.s2 = s1, s2
}

// clip, clamps value.
func clip(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// changeExt,  ".adx" -> ".wav"
func changeExt(fname, newExt string) string {
	ext := filepath.Ext(fname)
	return fname[:len(fname)-len(ext)] + newExt
}

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Printf("Usage: %s <input.adx> [output.wav]\n", os.Args[0])
		fmt.Println("If output path is not specified, auto generate .wav file")
		return
	}

	inPath := os.Args[1]
	var outPath string

	if len(os.Args) == 3 {
		outPath = os.Args[2]
	} else {
		outPath = changeExt(inPath, ".wav")
	}

	in, err := os.Open(inPath)
	if err != nil {
		fmt.Printf("Error: Can't open input: %s\n", err)
		return
	}
	defer in.Close()

	header := make([]byte, 16)
	_, err = in.Read(header)
	if err != nil {
		fmt.Printf("Error reading header: %s\n", err)
		return
	}

	channels := int(header[7])
	freq := int(binary.BigEndian.Uint32(header[8:12]))
	size := int(binary.BigEndian.Uint32(header[12:16]))
	offset := int(binary.BigEndian.Uint16(header[2:4]))
	offset -= 2 // that's it

	fmt.Printf("Channels: %d\nFreq: %d Hz\nSize: %d samples\nOffset: %d\n", channels, freq, size, offset)

	// check signature
	sig := make([]byte, 7)
	_, err = in.Seek(int64(offset), os.SEEK_SET)
	if err != nil {
		fmt.Println("Error seeking to signature:", err)
		return
	}
	_, err = in.Read(sig[1:])
	if err != nil {
		fmt.Println("Error reading signature:", err)
		return
	}
	sig[0] = 0x80

	if sig[0] != 0x80 || string(sig[1:]) != "(c)CRI" {
		fmt.Println("Error: Invalid ADX!")
		return
	}

	out, err := os.Create(outPath)
	if err != nil {
		fmt.Printf("Error: Can't create output: %s\n", err)
		return
	}
	defer out.Close()

	fmt.Printf("Converting %s -> %s\n", inPath, outPath)

	// calc coefs
	x := 500.0
	y := float64(freq)
	z := math.Cos(2.0 * math.Pi * x / y)
	a := math.Sqrt2 - z
	b := math.Sqrt2 - 1.0
	c := (a - math.Sqrt((a+b)*(a-b))) / b
	coef1 := int(math.Floor(8192.0 * c))
	coef2 := int(math.Floor(-4096.0 * c * c))

	// write wav header
	wavHdr := wavHeader{
		Riff:          [4]byte{'R', 'I', 'F', 'F'},
		Wave:          [4]byte{'W', 'A', 'V', 'E'},
		Fmt:           [4]byte{'f', 'm', 't', ' '},
		Data:          [4]byte{'d', 'a', 't', 'a'},
		FmtSize:       16,
		Format:        1,
		Channels:      uint16(channels),
		SampleRate:    uint32(freq),
		BitsPerSample: 16,
	}
	wavHdr.BlockAlign = uint16(channels) * (wavHdr.BitsPerSample / 8)
	wavHdr.ByteRate = uint32(freq) * uint32(wavHdr.BlockAlign)
	wavHdr.DataSize = uint32(size) * uint32(wavHdr.BlockAlign)
	wavHdr.Size = wavHdr.DataSize + uint32(binary.Size(wavHdr)) - 8

	err = binary.Write(out, binary.LittleEndian, &wavHdr)
	if err != nil {
		fmt.Println("Error writing WAV header:", err)
		return
	}

	// decode data
	prev := [2]prevSample{{0, 0}, {0, 0}}
	var buf []byte
	if channels == 1 {
		buf = make([]byte, 18) //mono
	} else {
		buf = make([]byte, 36)  //stereo
	}

	outbuf := make([]int16, 32*2) //max samples * 2

	for size > 0 {
		_, err := in.Read(buf)
		if err != nil {
			fmt.Println("Error reading ADX data:", err)
			return
		}

		wsize := 32
		if size < 32 {
			wsize = size // last block
		}

		if channels == 1 {
			adx2pcm(outbuf[:wsize*2], buf, &prev[0], coef1, coef2)
			err = binary.Write(out, binary.LittleEndian, outbuf[:wsize*2])
			if err != nil {
				fmt.Println("Error writing data:", err)
				return
			}
		} else {
			tmpbuf := make([]int16, 64) //double
			adx2pcm(tmpbuf[:32], buf, &prev[0], coef1, coef2)          //ch1
			adx2pcm(tmpbuf[32:], buf[18:], &prev[1], coef1, coef2) //ch2

			// interleave stereo
			for i := 0; i < 32; i++ {
				outbuf[i*2] = tmpbuf[i]
				outbuf[i*2+1] = tmpbuf[i+32]
			}
			err = binary.Write(out, binary.LittleEndian, outbuf[:wsize*2])
			if err != nil {
				fmt.Println("Error writing stereo:", err)
				return
			}
		}

		size -= wsize
	}

	fmt.Println("Done!")
}
