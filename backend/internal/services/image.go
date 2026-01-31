package services

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/draw"
)

const maxImageEdge = 1024

type ImageProcessor struct{}

func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{}
}

func (p *ImageProcessor) ResizeToMaxEdge(reader io.Reader, contentType string) ([]byte, string, error) {
	decoded, format, err := image.Decode(reader)
	if err != nil {
		return nil, "", err
	}

	width := decoded.Bounds().Dx()
	height := decoded.Bounds().Dy()
	if width == 0 || height == 0 {
		return nil, "", fmt.Errorf("invalid image dimensions")
	}

	newWidth := width
	newHeight := height
	longEdge := width
	if height > longEdge {
		longEdge = height
	}
	if longEdge > maxImageEdge {
		if width >= height {
			scale := float64(maxImageEdge) / float64(width)
			newWidth = maxImageEdge
			newHeight = int(float64(height) * scale)
		} else {
			scale := float64(maxImageEdge) / float64(height)
			newHeight = maxImageEdge
			newWidth = int(float64(width) * scale)
		}
	}

	resized := decoded
	if newWidth != width || newHeight != height {
		canvas := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
		draw.CatmullRom.Scale(canvas, canvas.Bounds(), decoded, decoded.Bounds(), draw.Over, nil)
		resized = canvas
	}

	buffer := &bytes.Buffer{}
	encodeFormat := strings.ToLower(format)
	if encodeFormat == "png" || strings.Contains(contentType, "png") {
		if err := png.Encode(buffer, resized); err != nil {
			return nil, "", err
		}
		return buffer.Bytes(), "image/png", nil
	}

	if err := jpeg.Encode(buffer, resized, &jpeg.Options{Quality: 90}); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), "image/jpeg", nil
}

// ResizeToMaxEdgeFromBytes is like ResizeToMaxEdge but accepts a byte slice
func (p *ImageProcessor) ResizeToMaxEdgeFromBytes(data []byte, contentType string) ([]byte, string, error) {
	return p.ResizeToMaxEdge(bytes.NewReader(data), contentType)
}
