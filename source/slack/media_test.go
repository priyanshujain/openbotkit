package slack

import (
	"context"
	"errors"
	"testing"
)

type fileAPI struct {
	mockAPI
	file        *File
	err         error
	requestedID string
}

func (a *fileAPI) FilesInfo(_ context.Context, id string) (*File, error) {
	a.requestedID = id
	if a.err != nil {
		return nil, a.err
	}
	return a.file, nil
}

func TestResolveFileURL_URLPrivatePassesThrough(t *testing.T) {
	api := &fileAPI{}
	raw := "https://files.slack.com/files-pri/T436FEDD0-F0BLMEDEJCS/file__1_.jpg"

	got, err := ResolveFileURL(context.Background(), api, raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != raw {
		t.Errorf("got %q, want %q", got, raw)
	}
	if api.requestedID != "" {
		t.Errorf("url_private should not need files.info, looked up %q", api.requestedID)
	}
}

// A /files/… permalink serves an HTML page, so it must go through files.info.
func TestResolveFileURL_PermalinkLooksUpFileInfo(t *testing.T) {
	api := &fileAPI{file: &File{
		ID:         "F0BLMEDEJCS",
		URLPrivate: "https://files.slack.com/files-pri/T436FEDD0-F0BLMEDEJCS/file__1_.jpg",
	}}

	got, err := ResolveFileURL(context.Background(), api,
		"https://okcredit.slack.com/files/U02CMP7DXU1/F0BLMEDEJCS/file__1_.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if api.requestedID != "F0BLMEDEJCS" {
		t.Errorf("looked up %q", api.requestedID)
	}
	if got != api.file.URLPrivate {
		t.Errorf("got %q", got)
	}
}

func TestResolveFileURL_Errors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"bare file id", "F0BLMEDEJCS"},
		{"empty", ""},
		{"non-slack host", "https://example.com/files/U1/F123/x.png"},
		{"slack host without file path", "https://okcredit.slack.com/archives/C01/p1785334150236249"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveFileURL(context.Background(), &fileAPI{}, tc.raw); err == nil {
				t.Fatalf("expected error for %q", tc.raw)
			}
		})
	}
}

func TestResolveFileURL_FileInfoError(t *testing.T) {
	api := &fileAPI{err: errors.New("file_not_found")}
	_, err := ResolveFileURL(context.Background(), api,
		"https://okcredit.slack.com/files/U02CMP7DXU1/F0BLMEDEJCS/file.jpg")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveFileURL_NoDownloadableURL(t *testing.T) {
	api := &fileAPI{file: &File{ID: "F1"}}
	_, err := ResolveFileURL(context.Background(), api,
		"https://okcredit.slack.com/files/U02CMP7DXU1/F1/file.jpg")
	if err == nil {
		t.Fatal("expected error when the file has no url_private")
	}
}
