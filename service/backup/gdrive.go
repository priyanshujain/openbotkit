package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

var errNotFound = errors.New("not found")

type GDriveBackend struct {
	srv      *drive.Service
	folderID string
}

func NewGDriveBackend(ctx context.Context, httpClient *http.Client, folderID string) (*GDriveBackend, error) {
	srv, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create drive service: %w", err)
	}
	return &GDriveBackend{srv: srv, folderID: folderID}, nil
}

func (b *GDriveBackend) Put(ctx context.Context, key string, r io.Reader, _ int64) error {
	name := key
	parentID := b.folderID

	parts := strings.Split(key, "/")
	if len(parts) > 1 {
		var err error
		parentID, err = b.ensureFolderPath(ctx, b.folderID, parts[:len(parts)-1])
		if err != nil {
			return fmt.Errorf("create folder path for %s: %w", key, err)
		}
		name = parts[len(parts)-1]
	}

	existing, err := b.findFile(ctx, parentID, name)
	if err != nil {
		return err
	}

	if existing != "" {
		_, err = b.srv.Files.Update(existing, &drive.File{}).
			Context(ctx).
			Media(r).
			Do()
	} else {
		_, err = b.srv.Files.Create(&drive.File{
			Name:    name,
			Parents: []string{parentID},
		}).Context(ctx).
			Media(r).
			Do()
	}
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (b *GDriveBackend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	fileID, err := b.resolveKey(ctx, key)
	if err != nil {
		return nil, err
	}
	resp, err := b.srv.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	return resp.Body, nil
}

func (b *GDriveBackend) Head(ctx context.Context, key string) (bool, error) {
	_, err := b.resolveKey(ctx, key)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (b *GDriveBackend) List(ctx context.Context, prefix string) ([]string, error) {
	folderID := b.folderID

	parts := strings.Split(prefix, "/")
	if len(parts) > 0 && prefix != "" {
		var err error
		folderID, err = b.resolveFolderPath(ctx, b.folderID, parts)
		if err != nil {
			return nil, nil
		}
	}

	return b.listRecursive(ctx, folderID, prefix)
}

func (b *GDriveBackend) Delete(ctx context.Context, key string) error {
	fileID, err := b.resolveKey(ctx, key)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	return b.srv.Files.Delete(fileID).Context(ctx).Do()
}

func (b *GDriveBackend) resolveKey(ctx context.Context, key string) (string, error) {
	parts := strings.Split(key, "/")
	parentID := b.folderID

	for i, part := range parts {
		id, err := b.findFile(ctx, parentID, part)
		if err != nil {
			return "", err
		}
		if id == "" {
			return "", fmt.Errorf("%s: %w", key, errNotFound)
		}
		if i < len(parts)-1 {
			parentID = id
		} else {
			return id, nil
		}
	}
	return "", fmt.Errorf("%s: %w", key, errNotFound)
}

func (b *GDriveBackend) findFile(ctx context.Context, parentID, name string) (string, error) {
	q := fmt.Sprintf("'%s' in parents and name = '%s' and trashed = false", escapeDriveQuery(parentID), escapeDriveQuery(name))
	list, err := b.srv.Files.List().
		Q(q).
		Fields("files(id)").
		PageSize(1).
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("find %s: %w", name, err)
	}
	if len(list.Files) == 0 {
		return "", nil
	}
	return list.Files[0].Id, nil
}

func (b *GDriveBackend) ensureFolderPath(ctx context.Context, parentID string, parts []string) (string, error) {
	current := parentID
	for _, part := range parts {
		id, err := b.findFile(ctx, current, part)
		if err != nil {
			return "", err
		}
		if id == "" {
			f, err := b.srv.Files.Create(&drive.File{
				Name:     part,
				Parents:  []string{current},
				MimeType: "application/vnd.google-apps.folder",
			}).Context(ctx).Fields("id").Do()
			if err != nil {
				return "", fmt.Errorf("create folder %s: %w", part, err)
			}
			id = f.Id
		}
		current = id
	}
	return current, nil
}

func (b *GDriveBackend) resolveFolderPath(ctx context.Context, parentID string, parts []string) (string, error) {
	current := parentID
	for _, part := range parts {
		id, err := b.findFile(ctx, current, part)
		if err != nil {
			return "", err
		}
		if id == "" {
			return "", fmt.Errorf("folder %s: %w", part, errNotFound)
		}
		current = id
	}
	return current, nil
}

func (b *GDriveBackend) listRecursive(ctx context.Context, folderID, prefix string) ([]string, error) {
	var keys []string
	q := fmt.Sprintf("'%s' in parents and trashed = false", escapeDriveQuery(folderID))
	err := b.srv.Files.List().
		Q(q).
		Fields("files(id, name, mimeType)").
		Context(ctx).
		Pages(ctx, func(list *drive.FileList) error {
			for _, f := range list.Files {
				path := prefix
				if path != "" && !strings.HasSuffix(path, "/") {
					path += "/"
				}
				path += f.Name

				if f.MimeType == "application/vnd.google-apps.folder" {
					sub, err := b.listRecursive(ctx, f.Id, path)
					if err != nil {
						return err
					}
					keys = append(keys, sub...)
				} else {
					keys = append(keys, path)
				}
			}
			return nil
		})
	return keys, err
}

func FindOrCreateDriveFolder(ctx context.Context, httpClient *http.Client, folderName string) (string, error) {
	srv, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return "", fmt.Errorf("create drive service: %w", err)
	}

	q := fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder' and trashed = false", escapeDriveQuery(folderName))
	list, err := srv.Files.List().Q(q).Fields("files(id)").PageSize(1).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("search for folder: %w", err)
	}
	if len(list.Files) > 0 {
		return list.Files[0].Id, nil
	}

	f, err := srv.Files.Create(&drive.File{
		Name:     folderName,
		MimeType: "application/vnd.google-apps.folder",
	}).Context(ctx).Fields("id").Do()
	if err != nil {
		return "", fmt.Errorf("create folder: %w", err)
	}
	return f.Id, nil
}

// escapeDriveQuery escapes single quotes for Drive API query strings.
func escapeDriveQuery(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

var _ Backend = (*GDriveBackend)(nil)
