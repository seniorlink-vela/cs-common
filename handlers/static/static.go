package static

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

var (
	staticURLs map[string]FileDef
	pathPrefix string
	indexPage  string
)

type FileDef struct {
	MimeType string
	Contents string
	Path     string
	IsBinary bool
}

func (fd *FileDef) LoadContents() {
	contents, _ := ioutil.ReadFile(fd.Path)
	fd.Path = strings.TrimPrefix(fd.Path, pathPrefix)

	if strings.HasPrefix(fd.MimeType, "text") {
		fd.Contents = fmt.Sprintf("%s", contents)
		fd.IsBinary = false
	} else {
		fd.Contents = base64.StdEncoding.EncodeToString(contents)
		fd.IsBinary = true
	}
}

func walkDirectory(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	// find out if it's a dir or file, if file, register for handler
	if !info.IsDir() {
		fd := &FileDef{
			MimeType: mime.TypeByExtension(filepath.Ext(path)),
			Path:     path,
		}
		fd.LoadContents()
		staticURLs[fd.Path] = *fd
		if strings.HasSuffix(fd.Path, indexPage) {
			index := &FileDef{
				MimeType: fd.MimeType,
				Path:     strings.TrimSuffix(fd.Path, indexPage),
				Contents: fd.Contents,
				IsBinary: fd.IsBinary,
			}
			staticURLs[index.Path] = *index
			index2 := &FileDef{
				MimeType: fd.MimeType,
				Path:     strings.TrimSuffix(fd.Path, fmt.Sprintf("/%s", indexPage)),
				Contents: fd.Contents,
				IsBinary: fd.IsBinary,
			}
			staticURLs[index2.Path] = *index2
		}
	}
	return nil
}

// Walk through the static asset tree, and register any files found for the request list.
func LoadDirectoryTree(basePath, prefix, index string) error {
	pathPrefix = prefix
	indexPage = index
	staticURLs = map[string]FileDef{}
	return filepath.Walk(basePath, walkDirectory)
}

func HandleStaticALB(ctx context.Context, req events.ALBTargetGroupRequest) (*events.ALBTargetGroupResponse, error) {

	// We deliberately only accept `GET` requests for static assets
	if req.HTTPMethod == http.MethodGet {
		fd, ok := staticURLs[req.Path]

		if ok {
			resp := &events.ALBTargetGroupResponse{
				StatusCode:        http.StatusOK,
				StatusDescription: http.StatusText(http.StatusOK),
				Body:              fd.Contents,
				IsBase64Encoded:   fd.IsBinary,
				Headers: map[string]string{
					"Content-Type":  fd.MimeType,
					"Cache-Control": "public, max-age=604800, immutable",
				},
			}
			return resp, nil
		}
	}
	// This returns a `nil` error when the path isn't found, as this is by design meant
	// to be called before any other path handling.  The assumption is that any path not
	// found here is being handled by another handler
	return nil, nil
}
