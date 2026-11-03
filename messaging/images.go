package messaging

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// SetImagesDir sets where messages' images are kept:
// <dir>/<message id>/<name> (ADR 0005 "Images"). Unset, a post with images
// is refused. Call it before the service is used.
func (s *Service) SetImagesDir(dir string) {
	s.mu.Lock()
	s.imagesDir = dir
	s.mu.Unlock()
}

func (s *Service) imagesRoot() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.imagesDir
}

// pendingImage is an upload checked and named, ready to be written once
// the message has its id.
type pendingImage struct {
	image msgmodels.Image
	data  []byte
}

// prepareImages checks a post's uploads and gives each its stored name.
func (s *Service) prepareImages(uploads []msgmodels.ImageUpload) ([]pendingImage, error) {
	if len(uploads) == 0 {
		return nil, nil
	}

	err := msgmodels.ValidateImages(uploads)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if s.imagesRoot() == "" {
		return nil, ez.New(ez.EUNAVAILABLE, "this agc does not keep message images: post without images", nil)
	}

	taken := map[string]bool{}
	pending := make([]pendingImage, 0, len(uploads))

	for _, upload := range uploads {
		imageType, err := msgmodels.ImageType(upload.Name, upload.Data)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		name := msgmodels.ImageName(upload.Name, imageType, taken)
		taken[name] = true

		image := msgmodels.Image{Name: name, Type: imageType, Size: int64(len(upload.Data))}
		pending = append(pending, pendingImage{image: image, data: upload.Data})
	}

	return pending, nil
}

// writeImages keeps a stored message's images under its id.
func (s *Service) writeImages(messageID int64, pending []pendingImage) error {
	if len(pending) == 0 {
		return nil
	}

	dir := filepath.Join(s.imagesRoot(), strconv.FormatInt(messageID, 10))

	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return ez.Wrap(err)
	}

	for _, p := range pending {
		err = os.WriteFile(filepath.Join(dir, p.image.Name), p.data, 0o644)
		if err != nil {
			_ = os.RemoveAll(dir)
			return ez.Wrap(err)
		}
	}

	return nil
}

// OpenImage opens the image called name of message messageID for a caller
// who may read the message's channel; the caller closes it. Not found when
// the message, the image or its file is absent.
func (s *Service) OpenImage(ctx context.Context, caller Caller, messageID int64, name string) (*os.File, *msgmodels.Image, error) {
	notFound := ez.New(ez.ENOTFOUND, fmt.Sprintf("message %d has no image %s", messageID, name), nil)

	err := checkCaller(caller)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	m := &msgmodels.Message{}

	err = s.db.NewSelect().Model(m).Where("id = ?", messageID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, notFound
	}
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	_, err = s.checkRead(caller, []string{m.Channel})
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	var image *msgmodels.Image
	for i := range m.Images {
		if m.Images[i].Name == name {
			image = &m.Images[i]
		}
	}

	root := s.imagesRoot()
	if image == nil || root == "" {
		return nil, nil, notFound
	}

	// The name is one the message lists, which agc cleaned on post, so it
	// never leaves the message's directory.
	file, err := os.Open(filepath.Join(root, strconv.FormatInt(messageID, 10), image.Name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, notFound
	}
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	return file, image, nil
}
