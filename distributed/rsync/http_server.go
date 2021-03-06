// Package rsync adds file server and copying client to copy files
// between glow driver and agent.
package rsync

import (
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chrislusf/gleam/util"
)

type FileResource struct {
	FullPath     string `json:"path,omitempty"`
	TargetFolder string `json:"targetFolder,omitempty"`
}

type FileHash struct {
	FullPath     string `json:"path,omitempty"`
	TargetFolder string `json:"targetFolder,omitempty"`
	File         string `json:"file,omitempty"`
	Hash         uint32 `json:"hash,omitempty"`
}

type RsyncServer struct {
	Ip           string
	Port         int
	listenOn     string
	RelatedFiles []FileResource

	fileHashes []FileHash
}

func NewRsyncServer(relatedFiles ...FileResource) (*RsyncServer, error) {
	rs := &RsyncServer{
		RelatedFiles: relatedFiles,
	}
	for _, f := range rs.RelatedFiles {
		if fh, err := GenerateFileHash(f.FullPath); err != nil {
			log.Printf("Failed2 to read %s: %v", f, err)
		} else {
			fh.TargetFolder = f.TargetFolder
			rs.fileHashes = append(rs.fileHashes, *fh)
		}
	}
	return rs, nil
}

func (rs *RsyncServer) listHandler(w http.ResponseWriter, r *http.Request) {
	util.Json(w, r, http.StatusAccepted, ListFileResult{rs.fileHashes})
}

func (rs *RsyncServer) fileHandler(w http.ResponseWriter, r *http.Request) {
	fileHash := r.URL.Path[len("/file/"):]
	for _, fh := range rs.fileHashes {
		if fmt.Sprintf("%d", fh.Hash) == fileHash {
			file, err := os.Open(fh.FullPath)
			if err != nil {
				log.Printf("Can not read file: %s", fh.FullPath)
				return
			}
			defer file.Close()
			http.ServeContent(w, r, fh.File, time.Now(), file)
			return
		}
	}
}

// go start a http server locally that will respond predictably to ranged requests
func (rs *RsyncServer) StartRsyncServer(listenOn string) {
	s := http.NewServeMux()
	s.HandleFunc("/list", rs.listHandler)
	s.HandleFunc("/file/", rs.fileHandler)

	var listener net.Listener
	var err error
	listener, err = net.Listen("tcp", listenOn)
	if err != nil {
		log.Fatal(err)
	}

	addr := listener.Addr().(*net.TCPAddr)
	rs.Ip = addr.String()[:strings.LastIndex(addr.String(), ":")]
	rs.Port = addr.Port

	go func() {
		http.Serve(listener, s)
	}()
}

func GenerateFileHash(fullpath string) (*FileHash, error) {

	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		return nil, err
	}

	f, err := os.Open(fullpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hasher := crc32.NewIEEE()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, err
	}
	crc := hasher.Sum32()

	return &FileHash{
		FullPath: fullpath,
		File:     filepath.Base(fullpath),
		Hash:     crc,
	}, nil
}
