package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	msgclient "github.com/vanclief/agent-composer/messaging/client"
)

const (
	defaultMsgURL       = "http://127.0.0.1:1202/api/msg"
	defaultMsgVsockPort = 5100
)

// msgState is what join saves so post and leave know who is speaking.
type msgState struct {
	ParticipantID string `json:"participant_id"`
	Name          string `json:"name"`
}

func msgCommand() *cli.Command {
	return &cli.Command{
		Name:  "msg",
		Usage: "Talk to other participants on agc channels (see /etc/agc/README.md)",
		Subcommands: []*cli.Command{
			{
				Name:      "join",
				Usage:     "Take a display name and print your participant id",
				ArgsUsage: "<name>",
				Action:    msgJoin,
			},
			{
				Name:   "leave",
				Usage:  "Release your name",
				Action: msgLeave,
			},
			{
				Name:      "post",
				Usage:     "Post a message to a channel",
				ArgsUsage: "<#channel> <text...>",
				Flags: []cli.Flag{
					&cli.Int64Flag{Name: "reply-to", Usage: "Id of the message this replies to"},
				},
				Action: msgPost,
			},
			{
				Name:      "ask",
				Usage:     "Post a question: it shows as waiting on the channel's readers until someone else replies to it",
				ArgsUsage: "<#channel> <text...>",
				Action:    msgAsk,
			},
			{
				Name:      "read",
				Usage:     "Print messages after a cursor, oldest first",
				ArgsUsage: "<#channel>",
				Flags: []cli.Flag{
					&cli.Int64Flag{Name: "after", Usage: "Only messages with an id greater than this"},
					&cli.IntFlag{Name: "limit", Usage: "Maximum messages to return (default 50, max 500)"},
				},
				Action: msgRead,
			},
			{
				Name:      "wait",
				Usage:     "Block until a message after the cursor arrives in any of the channels, or the timeout passes",
				ArgsUsage: "<#channel>[,<#channel>...]",
				Flags: []cli.Flag{
					&cli.Int64Flag{Name: "after", Usage: "Only messages with an id greater than this"},
					&cli.DurationFlag{Name: "timeout", Usage: "How long to wait", Value: 30 * time.Second},
				},
				Action: msgWait,
			},
			{
				Name:   "channels",
				Usage:  "List the channels you may read",
				Action: msgChannels,
			},
		},
	}
}

func msgJoin(c *cli.Context) error {
	args, err := msgArgs(c, 1)
	if err != nil {
		return err
	}

	p, err := newMsgClient().Join(c.Context, args[0])
	if err != nil {
		return err
	}

	err = saveMsgState(msgState{ParticipantID: p.ID, Name: p.Name})
	if err != nil {
		return err
	}

	return printJSON(p)
}

func msgLeave(c *cli.Context) error {
	state, err := loadMsgState()
	if err != nil {
		return err
	}

	res, err := newMsgClient().Leave(c.Context, state.ParticipantID)
	if err != nil {
		return err
	}

	err = os.Remove(msgStatePath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ez.Wrap(err)
	}

	return printJSON(res)
}

func msgPost(c *cli.Context) error {
	args, err := msgArgs(c, 2)
	if err != nil {
		return err
	}

	state, err := loadMsgState()
	if err != nil {
		return err
	}

	var replyTo *int64
	if c.IsSet("reply-to") {
		id := c.Int64("reply-to")
		replyTo = &id
	}

	body := strings.Join(args[1:], " ")

	m, err := newMsgClient().Post(c.Context, state.ParticipantID, msgChannel(args[0]), body, replyTo)
	if err != nil {
		return err
	}

	return printJSON(m)
}

// msgAsk is msgPost for a question, over the same transport. Answer it
// with agc msg post --reply-to <its id>.
func msgAsk(c *cli.Context) error {
	args, err := msgArgs(c, 2)
	if err != nil {
		return err
	}

	state, err := loadMsgState()
	if err != nil {
		return err
	}

	body := strings.Join(args[1:], " ")

	m, err := newMsgClient().Ask(c.Context, state.ParticipantID, msgChannel(args[0]), body)
	if err != nil {
		return err
	}

	return printJSON(m)
}

func msgRead(c *cli.Context) error {
	args, err := msgArgs(c, 1)
	if err != nil {
		return err
	}

	res, err := newMsgClient().Read(c.Context, msgChannel(args[0]), c.Int64("after"), c.Int("limit"))
	if err != nil {
		return err
	}

	return printJSON(res)
}

func msgWait(c *cli.Context) error {
	args, err := msgArgs(c, 1)
	if err != nil {
		return err
	}

	channels := []string{}
	for _, channel := range strings.Split(strings.Join(args, ","), ",") {
		trimmed := strings.TrimSpace(channel)
		if trimmed != "" {
			channels = append(channels, msgChannel(trimmed))
		}
	}

	res, err := newMsgClient().WaitChannels(c.Context, channels, c.Int64("after"), c.Duration("timeout"))
	if err != nil {
		return err
	}

	return printJSON(res)
}

func msgChannels(c *cli.Context) error {
	res, err := newMsgClient().Channels(c.Context)
	if err != nil {
		return err
	}

	return printJSON(res)
}

// msgArgs returns the positional args, requiring at least min. urfave/cli
// v2 stops reading flags at the first positional arg, so flags written
// after it (`post '#ch' hi --reply-to 3`) are picked out here and set on
// the context. Only this command's flag names count, so a body such as
// "- fixed the test" stays text, and anything after `--` is text too.
func msgArgs(c *cli.Context, min int) ([]string, error) {
	known := map[string]bool{}
	for _, f := range c.Command.Flags {
		for _, name := range f.Names() {
			known[name] = true
		}
	}

	var positional []string
	args := c.Args().Slice()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") || !known[name] {
			positional = append(positional, arg)
			continue
		}

		if !hasValue {
			if i+1 == len(args) {
				return nil, ez.New(ez.EINVALID, "--"+name+" needs a value", nil)
			}

			i++
			value = args[i]
		}

		err := c.Set(name, value)
		if err != nil {
			return nil, ez.New(ez.EINVALID, "invalid value for --"+name+": "+value, err)
		}
	}

	if len(positional) < min {
		return nil, ez.New(ez.EINVALID, "usage: agc msg "+c.Command.Name+" "+c.Command.ArgsUsage, nil)
	}

	return positional, nil
}

// msgChannel adds the leading # when it was left off, since an unquoted
// #name starts a comment in most shells. A DM channel (`@<owner>/<name>`)
// is left as it is.
func msgChannel(channel string) string {
	if strings.HasPrefix(channel, "#") || strings.HasPrefix(channel, "@") {
		return channel
	}

	return "#" + channel
}

// newMsgClient picks the transport: AGC_MSG_URL if set, vsock to the
// host when running inside a Task VM, else agc's REST API on localhost.
func newMsgClient() *msgclient.Client {
	url := os.Getenv("AGC_MSG_URL")
	if url != "" {
		return &msgclient.Client{BaseURL: url, HTTP: http.DefaultClient}
	}

	_, err := os.Stat("/dev/vsock")
	if err == nil {
		port := uint32(defaultMsgVsockPort)
		parsed, err := strconv.ParseUint(os.Getenv("AGC_MSG_VSOCK_PORT"), 10, 32)
		if err == nil {
			port = uint32(parsed)
		}

		return &msgclient.Client{BaseURL: "http://vsock", HTTP: msgclient.DialVsock(port)}
	}

	return &msgclient.Client{BaseURL: defaultMsgURL, HTTP: http.DefaultClient}
}

func msgStatePath() string {
	path := os.Getenv("AGC_MSG_STATE")
	if path != "" {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".agc-msg.json"
	}

	return filepath.Join(home, ".agc-msg.json")
}

func saveMsgState(state msgState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.WriteFile(msgStatePath(), encoded, 0o600)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func loadMsgState() (msgState, error) {
	state := msgState{}

	data, err := os.ReadFile(msgStatePath())
	if errors.Is(err, os.ErrNotExist) {
		return state, ez.New(ez.EINVALID, "not joined: run agc msg join <name> first", err)
	}
	if err != nil {
		return state, ez.Wrap(err)
	}

	err = json.Unmarshal(data, &state)
	if err != nil || state.ParticipantID == "" {
		return state, ez.New(ez.EINVALID, "state file "+msgStatePath()+" is unreadable: run agc msg join <name> again", err)
	}

	return state, nil
}
