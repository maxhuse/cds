package nfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-gorp/gorp"
	"github.com/rockbears/log"
	gonfs "github.com/vmware/go-nfs-client/nfs"
	"github.com/vmware/go-nfs-client/nfs/rpc"

	"github.com/ovh/cds/engine/cdn/storage"
	"github.com/ovh/cds/engine/cdn/storage/encryption"
	"github.com/ovh/cds/engine/gorpmapper"
	"github.com/ovh/cds/sdk"
)

type Nfs struct {
	storage.AbstractUnit
	encryption.ConvergentEncryption
	config storage.NFSStorageConfiguration
}

var (
	_ storage.StorageUnit = new(Nfs)
)

func init() {
	storage.RegisterDriver("nfs", new(Nfs))
}

type Reader struct {
	ctx       context.Context
	dialMount *gonfs.Mount
	target    *gonfs.Target
	reader    io.ReadCloser
}

func (r *Reader) Close() error {
	var firstError error
	if err := r.reader.Close(); err != nil {
		firstError = err
		log.Error(r.ctx, "reader: unable to close file")
	}
	if err := r.target.Close(); err != nil {
		log.Error(r.ctx, "reader: unable to close mount")
		if firstError == nil {
			firstError = err
		}
	}
	if err := r.dialMount.Close(); err != nil {
		log.Error(r.ctx, "reader: unable to close DialMount ")
		if firstError == nil {
			firstError = err
		}
	}
	return sdk.WithStack(firstError)
}

func (r *Reader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

type Writer struct {
	ctx       context.Context
	dialMount *gonfs.Mount
	target    *gonfs.Target
	writer    io.WriteCloser
}

func (w *Writer) Close() error {
	var firstError error
	if err := w.writer.Close(); err != nil {
		firstError = err
		log.Error(w.ctx, "writer: unable to close file")
	}
	if err := w.target.Close(); err != nil {
		log.Error(w.ctx, "writer: unable to close mount")
		if firstError == nil {
			firstError = err
		}
	}
	if err := w.dialMount.Close(); err != nil {
		log.Error(w.ctx, "writer: unable to close DialMount ")
		if firstError == nil {
			firstError = err
		}
	}
	return sdk.WithStack(firstError)
}

func (w *Writer) Write(p []byte) (n int, err error) {
	return w.writer.Write(p)
}

func (n *Nfs) Connect() (*gonfs.Mount, *gonfs.Target, error) {
	dialMount, err := gonfs.DialMount(n.config.Host)
	if err != nil {
		return nil, nil, sdk.WrapError(err, "unable to dial mount")
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, nil, sdk.WrapError(err, "unable to get hostname")
	}
	auth := rpc.NewAuthUnix(hostname, n.config.UserID, n.config.GroupID)
	v, err := dialMount.Mount(n.config.TargetPartition, auth.Auth())
	if err != nil {
		return nil, nil, sdk.WrapError(err, "unable to mount volume %s", n.config.TargetPartition)
	}
	return dialMount, v, nil
}

func (n *Nfs) Init(_ context.Context, cfg interface{}) error {
	config, is := cfg.(*storage.NFSStorageConfiguration)
	if !is {
		return sdk.WithStack(fmt.Errorf("invalid configuration: %T", cfg))
	}
	n.config = *config
	n.ConvergentEncryption = encryption.New(config.Encryption)

	_, target, err := n.Connect()
	if err != nil {
		return err
	}

	// Init subpath
	if _, err := target.Mkdir(config.SubPath, os.FileMode(0700)); err != nil {
		if !os.IsExist(err) {
			return sdk.WrapError(err, "unable to create subpath")
		}
	}
	return nil
}

func (n *Nfs) ItemExists(ctx context.Context, m *gorpmapper.Mapper, db gorp.SqlExecutor, i sdk.CDNItem) (bool, error) {
	dial, target, err := n.Connect()
	if err != nil {
		return false, err
	}
	defer dial.Close()   //nolint
	defer target.Close() //nolint

	iu, err := n.ExistsInDatabase(ctx, m, db, i.ID)
	if err != nil {
		if sdk.ErrorIs(err, sdk.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	// Lookup on the filesystem according to the locator
	path, err := n.filename(target, *iu)
	if err != nil {
		return false, err
	}
	finfo, _, err := target.Lookup(path)
	if err != nil {
		return false, sdk.WithStack(err)
	}
	return finfo.Name() != "", nil
}

func (n *Nfs) Status(_ context.Context) []sdk.MonitoringStatusLine {
	var lines []sdk.MonitoringStatusLine
	return lines
}

func (n *Nfs) Remove(ctx context.Context, i sdk.CDNItemUnit) error {
	dial, target, err := n.Connect()
	if err != nil {
		return err
	}
	defer dial.Close()   //nolint
	defer target.Close() //nolint

	path, err := n.filename(target, i)
	if err != nil {
		return err
	}
	log.Debug(ctx, "[%T] remove %s", n, path)
	if err := target.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return sdk.ErrNotFound
		}
		return sdk.WithStack(err)
	}
	return nil
}

func (n *Nfs) NewReader(ctx context.Context, i sdk.CDNItemUnit) (io.ReadCloser, error) {
	dial, target, err := n.Connect()
	if err != nil {
		return nil, err
	}

	// Open the file from the filesystem according to the locator
	path, err := n.filename(target, i)
	if err != nil {
		return nil, sdk.WithStack(err)
	}
	log.Debug(ctx, "[%T] reading from %s", n, path)
	f, err := target.Open(path)

	nfsReader := &Reader{ctx: ctx, dialMount: dial, target: target, reader: f}

	return nfsReader, sdk.WithStack(err)
}

func (n *Nfs) NewWriter(ctx context.Context, i sdk.CDNItemUnit) (io.WriteCloser, error) {
	dial, target, err := n.Connect()
	if err != nil {
		return nil, err
	}

	// Open the file from the filesystem according to the locator
	path, err := n.filename(target, i)
	if err != nil {
		return nil, err
	}
	log.Debug(ctx, "[%T] writing to %s", n, path)

	f, err := target.OpenFile(path, os.FileMode(0640))
	if err != nil {
		return nil, sdk.WithStack(err)
	}

	return &Writer{ctx: ctx, dialMount: dial, target: target, writer: f}, nil
}

func (n *Nfs) filename(target *gonfs.Target, i sdk.CDNItemUnit) (string, error) {
	loc := i.Locator
	if _, err := target.Mkdir(filepath.Join(n.config.SubPath, loc[:3]), os.FileMode(0775)); err != nil {
		if !os.IsExist(err) {
			return "", sdk.WithStack(err)
		}
	}
	return filepath.Join(n.config.SubPath, loc[:3], loc), nil
}
