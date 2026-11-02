package sandboxes

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/task"
)

// EnvVolumeRoot overrides where msb keeps named volumes on this host.
const EnvVolumeRoot = "AGC_MSB_VOLUMES_DIR"

// transcriptDirs are the harness session directories on a sandbox volume,
// relative to its root (the guest's task.HarnessDir): Claude Code's
// projects/<dir>/<session>.jsonl and Codex's sessions/YYYY/MM/DD/*.jsonl.
var transcriptDirs = []string{".claude/projects", ".codex/sessions"}

// VolumeRoot is the host directory holding msb's named volumes, one
// directory each: $AGC_MSB_VOLUMES_DIR, else $MSB_HOME/volumes, else
// ~/.microsandbox/volumes, msb's default.
func VolumeRoot() string {
	root := os.Getenv(EnvVolumeRoot)
	if root != "" {
		return root
	}

	msbHome := os.Getenv("MSB_HOME")
	if msbHome != "" {
		return filepath.Join(msbHome, "volumes")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".microsandbox", "volumes")
	}

	return filepath.Join(home, ".microsandbox", "volumes")
}

// volumeExists reports whether the named volume's directory is on the host.
func (api *API) volumeExists(volume string) (bool, error) {
	_, err := os.Stat(filepath.Join(api.volumeRoot, volume))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, ez.New(ez.EINTERNAL, "cannot read volume "+volume, err)
	}

	return true, nil
}

// Transcript is one harness session file on a sandbox volume. Path is
// relative to the volume, which the guest sees at task.HarnessDir.
type Transcript struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

type ListTranscriptsRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r ListTranscriptsRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

type ListTranscriptsResponse struct {
	Volume      string       `json:"volume"`
	Transcripts []Transcript `json:"transcripts"`
}

// ListTranscripts lists the harness session files on the sandbox's volume,
// newest first, read on the host and never through the VM, so it works for
// stopped sandboxes and, by name, for deleted ones.
func (api *API) ListTranscripts(ctx context.Context, requester interface{}, request *ListTranscriptsRequest) (*ListTranscriptsResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	volume, err := api.findVolume(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	root, err := os.OpenRoot(filepath.Join(api.volumeRoot, volume))
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot open volume "+volume, err)
	}
	defer root.Close()

	transcripts := []Transcript{}
	for _, dir := range transcriptDirs {
		// WalkDir does not follow links, and Root keeps every read inside
		// the volume, so a link the guest planted leads nowhere.
		err = fs.WalkDir(root.FS(), dir, func(name string, entry fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) && name == dir {
				return fs.SkipDir
			}
			if err != nil {
				return err
			}
			if !entry.Type().IsRegular() || !strings.HasSuffix(name, ".jsonl") {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return err
			}

			transcripts = append(transcripts, Transcript{Path: name, Size: info.Size(), ModifiedAt: info.ModTime().UTC()})
			return nil
		})
		if err != nil {
			return nil, ez.New(ez.EINTERNAL, "cannot list "+dir+" on volume "+volume, err)
		}
	}

	sort.SliceStable(transcripts, func(i, j int) bool {
		return transcripts[i].ModifiedAt.After(transcripts[j].ModifiedAt)
	})

	return &ListTranscriptsResponse{Volume: volume, Transcripts: transcripts}, nil
}

type GetTranscriptRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Path string    `json:"path"`
}

// Validate confines Path to the transcript directories of the volume: a
// relative, clean path with no "..", "." or empty segment.
func (r GetTranscriptRequest) Validate() error {
	err := validateRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	if r.Path == "" {
		return ez.New(ez.EINVALID, "path is required", nil)
	}

	if !fs.ValidPath(r.Path) || strings.Contains(r.Path, `\`) || path.Clean(r.Path) != r.Path {
		return ez.New(ez.EINVALID, "path "+r.Path+" must be relative to the volume, without .. or empty segments", nil)
	}

	for _, dir := range transcriptDirs {
		if strings.HasPrefix(r.Path, dir+"/") {
			return nil
		}
	}

	return ez.New(ez.EINVALID, "path "+r.Path+" is not under "+strings.Join(transcriptDirs, " or "), nil)
}

// TranscriptFile is an open transcript. Close it when done.
type TranscriptFile struct {
	io.ReadSeekCloser
	Name       string
	Size       int64
	ModifiedAt time.Time
}

// OpenTranscript opens one transcript on the sandbox's volume for reading,
// on the host, never through the VM. A link, even one inside the volume,
// is refused: transcripts are regular files.
func (api *API) OpenTranscript(ctx context.Context, requester interface{}, request *GetTranscriptRequest) (*TranscriptFile, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	volume, err := api.findVolume(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	root, err := os.OpenRoot(filepath.Join(api.volumeRoot, volume))
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot open volume "+volume, err)
	}
	defer root.Close()

	info, err := root.Lstat(request.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ez.New(ez.ENOTFOUND, "transcript "+request.Path+" not found on volume "+volume, err)
	}
	if err != nil {
		return nil, ez.New(ez.EINVALID, "cannot read transcript "+request.Path+" on volume "+volume, err)
	}
	if !info.Mode().IsRegular() {
		return nil, ez.New(ez.EINVALID, "transcript "+request.Path+" is not a regular file", nil)
	}

	file, err := root.Open(request.Path)
	if err != nil {
		return nil, ez.New(ez.EINVALID, "cannot open transcript "+request.Path+" on volume "+volume, err)
	}

	return &TranscriptFile{ReadSeekCloser: file, Name: path.Base(request.Path), Size: info.Size(), ModifiedAt: info.ModTime().UTC()}, nil
}

// findVolume returns the volume of the sandbox named by id or name. A
// name no sandbox has any longer still finds its volume while the volume
// is on the host: deleting a sandbox keeps it.
func (api *API) findVolume(ctx context.Context, id uuid.UUID, name string) (string, error) {
	record, err := api.find(ctx, id, name)
	if err == nil && record.Volume == "" {
		return "", ez.New(ez.ENOTFOUND, "sandbox "+record.Name+" has no volume: it was booted before sandboxes had one", nil)
	}
	if err == nil {
		return record.Volume, nil
	}
	if ez.ErrorCode(err) != ez.ENOTFOUND || name == "" {
		return "", ez.Wrap(err)
	}

	volume := task.VolumeName(name)
	if !volumeNameOK(volume) {
		return "", ez.Wrap(err)
	}

	exists, statErr := api.volumeExists(volume)
	if statErr != nil {
		return "", ez.Wrap(statErr)
	}
	if !exists {
		return "", ez.New(ez.ENOTFOUND, "sandbox "+name+" not found, and no volume "+volume+" is left from it", err)
	}

	return volume, nil
}

// volumeNameOK keeps a name from the URL from leaving the volume root.
func volumeNameOK(volume string) bool {
	return fs.ValidPath(volume) && !strings.ContainsAny(volume, `/\`) && volume != "." && volume != ".."
}
