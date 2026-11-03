package messaging

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/vanclief/ez"
)

// A message carries at most MaxImages images of at most MaxImageBytes
// each (ADR 0005 "Images").
const (
	MaxImages     = 4
	MaxImageBytes = 5 << 20
)

// imageTypes are the image types a message may carry, with the extension
// a stored name gets.
var imageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var imageNameUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Image is one image a message carries: its stored file name, its type,
// its size in bytes and the URL it is served at. Only name, type and size
// are stored; the URL is filled in from the message id.
type Image struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	URL  string `json:"url,omitempty"`
}

// ImageUpload is an image as a post sends it: a file name, which agc
// cleans, and the bytes, base64 in JSON.
type ImageUpload struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// ImagePath is the URL path a message's image is served at.
func ImagePath(messageID int64, name string) string {
	return "/api/msg/images/" + strconv.FormatInt(messageID, 10) + "/" + url.PathEscape(name)
}

// ImageType sniffs data and answers its type when a message may carry
// it, else an error naming what is accepted.
func ImageType(name string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", ez.New(ez.EINVALID, fmt.Sprintf("image %q is empty", name), nil)
	}

	if len(data) > MaxImageBytes {
		msg := fmt.Sprintf("image %q is %d bytes, at most %d (5 MB)", name, len(data), MaxImageBytes)
		return "", ez.New(ez.EINVALID, msg, nil)
	}

	sniffed := http.DetectContentType(data)
	_, ok := imageTypes[sniffed]
	if !ok {
		msg := fmt.Sprintf("image %q is %s: send a PNG, JPEG, GIF or WebP", name, sniffed)
		return "", ez.New(ez.EINVALID, msg, nil)
	}

	return sniffed, nil
}

// ImageName cleans a sent file name into the stored one: letters, digits,
// dots, dashes and underscores, at most 64 characters, ending in the
// extension of its type, and different from every name in taken.
func ImageName(sent string, imageType string, taken map[string]bool) string {
	ext := imageTypes[imageType]

	base := filepath.Base(strings.ReplaceAll(sent, "\\", "/"))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.Trim(imageNameUnsafe.ReplaceAllString(base, "-"), ".-")
	if base == "" {
		base = "image"
	}
	if len(base) > 56 {
		base = base[:56]
	}

	name := base + ext
	for n := 2; taken[name]; n++ {
		name = fmt.Sprintf("%s-%d%s", base, n, ext)
	}

	return name
}

// ValidateImages checks a post's images before anything is stored.
func ValidateImages(images []ImageUpload) error {
	if len(images) > MaxImages {
		msg := fmt.Sprintf("a message carries at most %d images, this one has %d", MaxImages, len(images))
		return ez.New(ez.EINVALID, msg, nil)
	}

	for _, image := range images {
		_, err := ImageType(image.Name, image.Data)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}
