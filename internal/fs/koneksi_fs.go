package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/koneksi/koneksi-drive/internal/api"
	"github.com/koneksi/koneksi-drive/internal/config"
)

type KoneksiFS struct {
	root   *koneksiNode
	client *api.Client
	cfg    *config.Config
	server *fuse.Server
	mu     sync.RWMutex
}

type koneksiNode struct {
	fs.Inode
	
	path     string
	info     *api.FileInfo
	client   *api.Client
	cfg      *config.Config
	mu       sync.RWMutex
	children map[string]*koneksiNode
}

func NewKoneksiFS(cfg *config.Config) (*KoneksiFS, error) {
	client, err := api.NewClient(&cfg.API)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	rootInfo := &api.FileInfo{
		Name:     "",
		IsDir:    true,
		Modified: time.Now(),
	}

	root := &koneksiNode{
		path:     "/",
		info:     rootInfo,
		client:   client,
		cfg:      cfg,
		children: make(map[string]*koneksiNode),
	}

	return &KoneksiFS{
		root:   root,
		client: client,
		cfg:    cfg,
	}, nil
}

func (kfs *KoneksiFS) Mount(mountpoint string) error {
	opts := &fuse.MountOptions{
		AllowOther: kfs.cfg.Mount.AllowOther,
		Debug:      false,
		FsName:     "koneksi",
		Name:       "koneksi-drive",
	}

	if kfs.cfg.Mount.ReadOnly {
		opts.Options = append(opts.Options, "ro")
	}

	server, err := fs.Mount(mountpoint, kfs.root, &fs.Options{
		MountOptions: *opts,
	})
	if err != nil {
		return fmt.Errorf("mount failed: %w", err)
	}

	kfs.server = server
	go server.Serve()
	
	return nil
}

func (kfs *KoneksiFS) Unmount() error {
	if kfs.server != nil {
		return kfs.server.Unmount()
	}
	return nil
}

// Implement fs.InodeEmbedder
var _ = (fs.InodeEmbedder)((*koneksiNode)(nil))

// Implement fs.NodeLookuper
var _ = (fs.NodeLookuper)((*koneksiNode)(nil))

func (n *koneksiNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	n.mu.RLock()
	child, ok := n.children[name]
	n.mu.RUnlock()

	if ok {
		n.setAttr(&out.Attr, child.info)
		return n.NewInode(ctx, child, n.stableAttr(child.info)), 0
	}

	// Try to fetch from API
	childPath := filepath.Join(n.path, name)
	files, err := n.client.List(n.path)
	if err != nil {
		return nil, syscall.ENOENT
	}

	for _, file := range files {
		if file.Name == name {
			child = &koneksiNode{
				path:     childPath,
				info:     &file,
				client:   n.client,
				cfg:      n.cfg,
				children: make(map[string]*koneksiNode),
			}

			n.mu.Lock()
			n.children[name] = child
			n.mu.Unlock()

			n.setAttr(&out.Attr, &file)
			return n.NewInode(ctx, child, n.stableAttr(&file)), 0
		}
	}

	return nil, syscall.ENOENT
}

// Implement fs.NodeReaddirer
var _ = (fs.NodeReaddirer)((*koneksiNode)(nil))

func (n *koneksiNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	if !n.info.IsDir {
		return nil, syscall.ENOTDIR
	}

	files, err := n.client.List(n.path)
	if err != nil {
		return nil, syscall.EIO
	}

	entries := make([]fuse.DirEntry, 0, len(files))
	
	n.mu.Lock()
	n.children = make(map[string]*koneksiNode)
	
	for _, file := range files {
		mode := uint32(syscall.S_IFREG)
		if file.IsDir {
			mode = syscall.S_IFDIR
		}
		
		entries = append(entries, fuse.DirEntry{
			Name: file.Name,
			Mode: mode,
		})

		child := &koneksiNode{
			path:     filepath.Join(n.path, file.Name),
			info:     &file,
			client:   n.client,
			cfg:      n.cfg,
			children: make(map[string]*koneksiNode),
		}
		n.children[file.Name] = child
	}
	n.mu.Unlock()

	return fs.NewListDirStream(entries), 0
}

// Implement fs.NodeGetattrer
var _ = (fs.NodeGetattrer)((*koneksiNode)(nil))

func (n *koneksiNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	n.setAttr(&out.Attr, n.info)
	return 0
}

// Implement fs.NodeOpener
var _ = (fs.NodeOpener)((*koneksiNode)(nil))

func (n *koneksiNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if n.info.IsDir {
		return nil, 0, syscall.EISDIR
	}

	if n.cfg.Mount.ReadOnly && (flags&(syscall.O_WRONLY|syscall.O_RDWR)) != 0 {
		return nil, 0, syscall.EROFS
	}

	return &koneksiFileHandle{node: n, flags: flags}, fuse.FOPEN_DIRECT_IO, 0
}

// Implement fs.NodeCreater
var _ = (fs.NodeCreater)((*koneksiNode)(nil))

func (n *koneksiNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	if n.cfg.Mount.ReadOnly {
		return nil, nil, 0, syscall.EROFS
	}

	childPath := filepath.Join(n.path, name)
	
	// Create empty file
	if err := n.client.Write(childPath, strings.NewReader("")); err != nil {
		return nil, nil, 0, syscall.EIO
	}

	info := &api.FileInfo{
		Name:     name,
		Size:     0,
		IsDir:    false,
		Modified: time.Now(),
		Path:     childPath,
	}

	child := &koneksiNode{
		path:     childPath,
		info:     info,
		client:   n.client,
		cfg:      n.cfg,
		children: make(map[string]*koneksiNode),
	}

	n.mu.Lock()
	n.children[name] = child
	n.mu.Unlock()

	n.setAttr(&out.Attr, info)
	inode := n.NewInode(ctx, child, n.stableAttr(info))
	fh := &koneksiFileHandle{node: child, flags: flags}

	return inode, fh, fuse.FOPEN_DIRECT_IO, 0
}

// Implement fs.NodeMkdirer
var _ = (fs.NodeMkdirer)((*koneksiNode)(nil))

func (n *koneksiNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if n.cfg.Mount.ReadOnly {
		return nil, syscall.EROFS
	}

	childPath := filepath.Join(n.path, name)
	
	if err := n.client.Mkdir(childPath); err != nil {
		return nil, syscall.EIO
	}

	info := &api.FileInfo{
		Name:     name,
		IsDir:    true,
		Modified: time.Now(),
		Path:     childPath,
	}

	child := &koneksiNode{
		path:     childPath,
		info:     info,
		client:   n.client,
		cfg:      n.cfg,
		children: make(map[string]*koneksiNode),
	}

	n.mu.Lock()
	n.children[name] = child
	n.mu.Unlock()

	n.setAttr(&out.Attr, info)
	return n.NewInode(ctx, child, n.stableAttr(info)), 0
}

// Implement fs.NodeUnlinker
var _ = (fs.NodeUnlinker)((*koneksiNode)(nil))

func (n *koneksiNode) Unlink(ctx context.Context, name string) syscall.Errno {
	if n.cfg.Mount.ReadOnly {
		return syscall.EROFS
	}

	childPath := filepath.Join(n.path, name)
	
	if err := n.client.Delete(childPath); err != nil {
		return syscall.EIO
	}

	n.mu.Lock()
	delete(n.children, name)
	n.mu.Unlock()

	return 0
}

// Implement fs.NodeRmdirer
var _ = (fs.NodeRmdirer)((*koneksiNode)(nil))

func (n *koneksiNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	return n.Unlink(ctx, name)
}

func (n *koneksiNode) setAttr(attr *fuse.Attr, info *api.FileInfo) {
	attr.Size = uint64(info.Size)
	attr.Mtime = uint64(info.Modified.Unix())
	attr.Ctime = attr.Mtime
	attr.Atime = attr.Mtime
	
	if info.IsDir {
		attr.Mode = syscall.S_IFDIR | 0755
	} else {
		attr.Mode = syscall.S_IFREG | 0644
	}
	
	attr.Uid = n.cfg.Mount.UID
	attr.Gid = n.cfg.Mount.GID
}

func (n *koneksiNode) stableAttr(info *api.FileInfo) fs.StableAttr {
	mode := uint32(syscall.S_IFREG)
	if info.IsDir {
		mode = syscall.S_IFDIR
	}
	return fs.StableAttr{
		Mode: mode,
	}
}

// File handle implementation
type koneksiFileHandle struct {
	node  *koneksiNode
	flags uint32
}

var _ = (fs.FileReader)((*koneksiFileHandle)(nil))

func (fh *koneksiFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	reader, err := fh.node.client.Read(fh.node.path)
	if err != nil {
		return nil, syscall.EIO
	}
	defer reader.Close()

	// Skip to offset
	if off > 0 {
		if _, err := io.CopyN(io.Discard, reader, off); err != nil {
			if err == io.EOF {
				return fuse.ReadResultData(nil), 0
			}
			return nil, syscall.EIO
		}
	}

	n, err := io.ReadFull(reader, dest)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, syscall.EIO
	}

	return fuse.ReadResultData(dest[:n]), 0
}

var _ = (fs.FileWriter)((*koneksiFileHandle)(nil))

func (fh *koneksiFileHandle) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	if fh.node.cfg.Mount.ReadOnly {
		return 0, syscall.EROFS
	}

	// For simplicity, we'll implement write as a full file replacement
	// A production implementation would handle partial writes properly
	tempFile, err := os.CreateTemp("", "koneksi-write-*")
	if err != nil {
		return 0, syscall.EIO
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// If offset is not 0, we need to read existing content first
	if off > 0 {
		reader, err := fh.node.client.Read(fh.node.path)
		if err != nil {
			return 0, syscall.EIO
		}
		defer reader.Close()

		if _, err := io.CopyN(tempFile, reader, off); err != nil && err != io.EOF {
			return 0, syscall.EIO
		}
	}

	// Write new data
	n, err := tempFile.Write(data)
	if err != nil {
		return 0, syscall.EIO
	}

	// Seek to beginning for upload
	if _, err := tempFile.Seek(0, 0); err != nil {
		return 0, syscall.EIO
	}

	// Upload file
	if err := fh.node.client.Write(fh.node.path, tempFile); err != nil {
		return 0, syscall.EIO
	}

	// Update file info
	fh.node.mu.Lock()
	fh.node.info.Size = off + int64(n)
	fh.node.info.Modified = time.Now()
	fh.node.mu.Unlock()

	return uint32(n), 0
}