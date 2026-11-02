package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/vanclief/agent-composer/core/resources/schedules"
)

// ListSchedules lists every schedule by name.
func (c *Client) ListSchedules(ctx context.Context) (*schedules.ListResponse, error) {
	out := &schedules.ListResponse{}
	err := c.do(ctx, http.MethodGet, "/api/schedules", nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// CreateSchedule stores a schedule; the server's scheduler runs it.
func (c *Client) CreateSchedule(ctx context.Context, request *schedules.CreateRequest) (*schedules.ScheduleView, error) {
	return c.scheduleCall(ctx, http.MethodPost, "/api/schedules", request)
}

// GetSchedule reads the schedule called ref (a name or an id).
func (c *Client) GetSchedule(ctx context.Context, ref string) (*schedules.ScheduleView, error) {
	return c.scheduleCall(ctx, http.MethodGet, schedulePath(ref, ""), nil)
}

// UpdateSchedule replaces the schedule called ref.
func (c *Client) UpdateSchedule(ctx context.Context, ref string, request *schedules.UpdateRequest) (*schedules.ScheduleView, error) {
	return c.scheduleCall(ctx, http.MethodPut, schedulePath(ref, ""), request)
}

// DeleteSchedule deletes the schedule and its items; its tasks stay.
func (c *Client) DeleteSchedule(ctx context.Context, ref string) (*schedules.DeleteResponse, error) {
	out := &schedules.DeleteResponse{}
	err := c.do(ctx, http.MethodDelete, schedulePath(ref, ""), nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// RunSchedule ticks the schedule now and returns the tasks it created.
func (c *Client) RunSchedule(ctx context.Context, ref string) (*schedules.RunResponse, error) {
	out := &schedules.RunResponse{}
	err := c.do(ctx, http.MethodPost, schedulePath(ref, "/run"), nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// ListScheduleItems lists the keys the schedule has seen, newest first.
func (c *Client) ListScheduleItems(ctx context.Context, ref string) (*schedules.ListItemsResponse, error) {
	out := &schedules.ListItemsResponse{}
	err := c.do(ctx, http.MethodGet, schedulePath(ref, "/items"), nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) scheduleCall(ctx context.Context, method string, path string, in any) (*schedules.ScheduleView, error) {
	out := &schedules.ScheduleView{}
	err := c.do(ctx, method, path, in, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func schedulePath(ref string, suffix string) string {
	return "/api/schedules/" + url.PathEscape(ref) + suffix
}
