package messages

import (
	"context"
	"fmt"
	"os"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// GetImageRequest names one image of a message (ADR 0005 "Images").
type GetImageRequest struct {
	MessageID int64  `json:"message_id"`
	Name      string `json:"name"`
}

func (r GetImageRequest) Validate() error {
	if r.MessageID <= 0 {
		return ez.New(ez.ENOTFOUND, fmt.Sprintf("no message %d", r.MessageID), nil)
	}

	if r.Name == "" {
		return ez.New(ez.ENOTFOUND, fmt.Sprintf("message %d has no image with an empty name", r.MessageID), nil)
	}

	return nil
}

// ImageResponse is an opened image with its stored name and type. The
// handler streams File as it is and closes it; it is never JSON.
type ImageResponse struct {
	File  *os.File        `json:"-"`
	Image msgmodels.Image `json:"image"`
}

// OpenImage opens a message's image for a person, who may read every
// channel (ADR 0005). Not found when the message, the image or its file is
// absent.
func (api *API) OpenImage(ctx context.Context, requester interface{}, request *GetImageRequest) (*ImageResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	file, image, err := api.msg.OpenImage(ctx, messaging.Caller{Human: true}, request.MessageID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &ImageResponse{File: file, Image: *image}, nil
}
